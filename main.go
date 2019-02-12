package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	cid "github.com/ipfs/go-cid"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/ipfs/ipfs-cluster/api"
	cluster "github.com/ipfs/ipfs-cluster/api/rest/client"
	ma "github.com/multiformats/go-multiaddr"

	hb "github.com/whyrusleeping/hellabot"
	log "gopkg.in/inconshreveable/log15.v2"
)

var prefix string
var gateway string
var replMin int
var replMax int
var bot *hb.Bot
var msgs chan msgWrap

type msgWrap struct {
	message string
	actor   string
}

var (
	cmdBotsnack    = "botsnack"
	cmdFriends     = "friends"
	cmdBefriend    = "befriend"
	cmdShun        = "shun"
	cmdPin         = "pin"
	cmdUnPin       = "unpin"
	cmdStatus      = "status"
	cmdOngoing     = "ongoing"
	cmdRecover     = "recover"
	cmdPinLegacy   = "legacypin"
	cmdUnpinLegacy = "legacyunpin"
)

var friends FriendsList

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func messageQueueProcess() {
	for mWrap := range msgs {
		sendMsg(mWrap.actor, mWrap.message)
	}
}

func botMsg(actor, msg string) {
	msgs <- msgWrap{
		message: msg,
		actor:   actor,
	}
}

func sendMsg(actor, msg string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			fmt.Println("recovered from panic, sleeping a bit")
			time.Sleep(10 * time.Second)
		}
	}()
	bot.Msg(actor, msg)
}

func formatError(action string, err error) error {
	if strings.Contains(err.Error(), "504 Gateway Time-out") {
		err = fmt.Errorf("504 Gateway Time-out")
	}
	if strings.Contains(err.Error(), "502 Bad Gateway") {
		err = fmt.Errorf("502 Bad Gateway")
	}

	return fmt.Errorf("%s failed: %s", action, err.Error())
}

func tryPin(path string, sh *shell.Shell) error {
	out, err := sh.Refs(path, true)
	if err != nil {
		return formatError("refs", err)
	}

	// throw away results
	for _ = range out {
	}

	err = sh.Pin(path)
	if err != nil {
		return formatError("pin", err)
	}

	return nil
}

func tryUnpin(path string, sh *shell.Shell) error {
	out, err := sh.Refs(path, true)
	if err != nil {
		return formatError("refs", err)
	}

	// throw away results
	for _ = range out {
	}

	err = sh.Unpin(path)
	if err != nil {
		return formatError("unpin", err)
	}

	return nil
}

var pinfile = "pins.log"

func writePin(pin, label string) error {
	fi, err := os.OpenFile(pinfile, os.O_APPEND|os.O_EXCL|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(fi, "%s\t%s\n", pin, label)
	if err != nil {
		return err
	}
	return fi.Close()
}

func Pin(b *hb.Bot, actor, path, label string) {
	if !strings.HasPrefix(path, "/ipfs") && !strings.HasPrefix(path, "/ipns") {
		path = "/ipfs/" + path
	}

	errs := make(chan error, len(shs))
	var wg sync.WaitGroup

	botMsg(actor, fmt.Sprintf("now pinning on %d nodes", len(shs)))

	// pin to every node concurrently.
	for i, sh := range shs {
		wg.Add(1)
		go func(i int, sh *shell.Shell) {
			defer wg.Done()
			if err := tryPin(path, sh); err != nil {
				errs <- fmt.Errorf("%s -- %s", shsUrls[i], err)
			}
		}(i, sh)
	}

	// close the err chan when done.
	go func() {
		wg.Wait()
		close(errs)
	}()

	// wait on the err chan and print every err we get as we get it.
	var failed int
	for err := range errs {
		botMsg(actor, err.Error())
		failed++
	}

	successes := len(shs) - failed
	botMsg(actor, fmt.Sprintf("pinned on %d of %d nodes (%d failures) -- %s%s",
		successes, len(shs), failed, gateway, path))

	if err := writePin(path, label); err != nil {
		botMsg(actor, fmt.Sprintf("failed to write log entry for last pin: %s", err))
	}
	clusterPinUnpin(b, actor, path, label, true)
}

func Unpin(b *hb.Bot, actor, path string) {
	if !strings.HasPrefix(path, "/ipfs") && !strings.HasPrefix(path, "/ipns") {
		path = "/ipfs/" + path
	}

	errs := make(chan error, len(shs))
	var wg sync.WaitGroup

	botMsg(actor, fmt.Sprintf("now unpinning on %d nodes", len(shs)))

	// pin to every node concurrently.
	for i, sh := range shs {
		wg.Add(1)
		go func(i int, sh *shell.Shell) {
			defer wg.Done()
			if err := tryUnpin(path, sh); err != nil {
				errs <- fmt.Errorf("%s -- %s", shsUrls[i], err)
			}
		}(i, sh)
	}

	// close the err chan when done.
	go func() {
		wg.Wait()
		close(errs)
	}()

	// wait on the err chan and print every err we get as we get it.
	var failed int
	for err := range errs {
		botMsg(actor, err.Error())
		failed++
	}

	successes := len(shs) - failed
	botMsg(actor, fmt.Sprintf("unpinned on %d of %d nodes (%d failures) -- %s%s",
		successes, len(shs), failed, gateway, path))
	clusterPinUnpin(b, actor, path, "", false)
}

func StatusCluster(b *hb.Bot, actor, path string) {
	ctx := context.Background()
	// pick random cluster
	i := rand.Intn(len(clusters))
	cluster := clusters[i]
	addr := clusterAddrs[i]

	c, err := resolveCid(path, shs[i], shsUrls[i])
	if err != nil {
		botMsg(actor, fmt.Sprintf("could not resolve cid: %s", err))
		return
	}

	st, err := cluster.Status(ctx, c, false)
	if err != nil {
		botMsg(actor, fmt.Sprintf("%s: error obtaining pin status: %s", addr, err))
		return
	}
	prettyClusterStatus(actor, st)
}

func StatusAllCluster(b *hb.Bot, actor string, filter api.TrackerStatus) {
	ctx := context.Background()
	// pick random cluster
	i := rand.Intn(len(clusters))
	cluster := clusters[i]
	addr := clusterAddrs[i]

	sts, err := cluster.StatusAll(ctx, filter, false)
	if err != nil {
		botMsg(actor, fmt.Sprintf("%s: error obtaining pin statuses: %s", addr, err))
		return
	}
	for _, st := range sts {
		prettyClusterStatus(actor, st)
		time.Sleep(5 * time.Second) // flood prevention
	}
	return
}

func prettyClusterStatus(actor string, st api.GlobalPinInfo) {
	botMsg(actor, fmt.Sprintf("Status for %s:", st.Cid))
	for _, info := range st.PeerMap {
		botMsg(actor, fmt.Sprintf("  - %s : %s | %s\n", info.PeerName, info.Status, info.Error))
	}
}

func PinCluster(b *hb.Bot, actor, path, label string) {
	clusterPinUnpin(b, actor, path, label, true)
	if err := writePin(path, label); err != nil {
		botMsg(actor, fmt.Sprintf("failed to write log entry for last pin: %s", err))
	}
}

func UnpinCluster(b *hb.Bot, actor, path string) {
	clusterPinUnpin(b, actor, path, "", false)
}

func RecoverCluster(b *hb.Bot, actor, path string) {
	ctx := context.Background()
	// pick up a random cluster
	i := rand.Intn(len(clusters))
	cluster := clusters[i]
	addr := clusterAddrs[i]

	botMsg(actor, fmt.Sprintf("Recovering pin via %s", addr))

	c, err := resolveCid(path, shs[i], shsUrls[i])
	if err != nil {
		botMsg(actor, fmt.Sprintf("could not determine cid to recover: %s", err))
		return
	}

	if c.String() != path {
		botMsg(actor, fmt.Sprintf("%s resolved as %s", path, c))
	}

	gpi, err := cluster.Recover(ctx, c, false)
	if err != nil {
		botMsg(actor, fmt.Sprintf("%s: failed to recover: %s", addr, err))
		return
	}
	botMsg(actor, fmt.Sprintf("Recover operation triggered for %s. You can later manually track the status with !status <cid>", c))
	prettyClusterStatus(actor, gpi)
	return
}

func resolveCid(path string, sh *shell.Shell, url string) (cid.Cid, error) {
	// fix path
	if !strings.HasPrefix(path, "/ipfs") && !strings.HasPrefix(path, "/ipns") {
		path = "/ipfs/" + path
	}

	parts := strings.Split(path, "/")

	// Only resolve path if /ipns or has subdirectories
	if strings.HasPrefix(path, "/ipns") || len(parts) > 3 {
		// resolve it
		cidstr, err := sh.ResolvePath(path)
		if err != nil {
			return cid.Undef, err
		}
		return cid.Decode(cidstr)
	}

	// parts is ["", "ipfs", "cid"]
	return cid.Decode(parts[2])
}

func waitForClusterOp(actor string, client cluster.Client, c cid.Cid, target api.TrackerStatus) {
	botMsg(actor, fmt.Sprintf("%s: operation submitted. Waiting for status to reach %s", c, target))

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	fp := cluster.StatusFilterParams{
		Cid:       c,
		Local:     false,
		Target:    target,
		CheckFreq: 5 * time.Second,
	}

	gpi, err := cluster.WaitFor(ctx, client, fp)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			botMsg(actor, fmt.Sprintf("%s: still not '%s'. I won't keep watching, but you can run !status <cid> to check manually.", c, target))
			return
		}
		botMsg(actor, fmt.Sprintf("%s: an error happened: %s. You can attempt recovery with !recover <cid>.", c, err))
		return
	}

	done := 0
	for _, info := range gpi.PeerMap {
		if info.Status == target {
			done++
		}
	}

	botMsg(actor, fmt.Sprintf("Reached %s in %d cluster peers: %s/ipfs/%s .", target, done, gateway, c))
}

func clusterPinUnpin(b *hb.Bot, actor, path, label string, pin bool) {
	ctx := context.Background()
	verb := "pin"
	if !pin {
		verb = "unpin"
	}

	// pick up a random cluster
	i := rand.Intn(len(clusters))
	cluster := clusters[i]
	addr := clusterAddrs[i]

	botMsg(actor, fmt.Sprintf("Cluster-%sning via %s", verb, addr))

	c, err := resolveCid(path, shs[i], shsUrls[i])
	if err != nil {
		botMsg(actor, fmt.Sprintf("could not determine cid to pin: %s", err))
		return
	}

	if c.String() != path {
		botMsg(actor, fmt.Sprintf("%s resolved as %s", path, c))
	}

	switch pin {
	case true:
		err = cluster.Pin(ctx, c, 0, 0, label)
		if err == nil {
			go waitForClusterOp(actor, cluster, c, api.TrackerStatusPinned)
		}
	case false:
		err = cluster.Unpin(ctx, c)
		if err == nil {
			go waitForClusterOp(actor, cluster, c, api.TrackerStatusUnpinned)
		}
	}

	if err != nil {
		botMsg(actor, fmt.Sprintf("%s: failed to %s in cluster: %s", addr, verb, err))
		return
	}
}

var shs []*shell.Shell
var shsUrls []string
var clusters []cluster.Client
var clusterAddrs []string

func loadHosts(file string) []string {
	fi, err := os.Open(file)
	if err != nil {
		fmt.Printf("failed to open %s hosts file, defaulting to localhost:5001\n", file)
		return []string{"localhost:5001"}
	}

	var hosts []string
	scan := bufio.NewScanner(fi)
	for scan.Scan() {
		hosts = append(hosts, scan.Text())
	}
	return hosts
}

func ensurePinLogExists() error {
	_, err := os.Stat(pinfile)
	if os.IsNotExist(err) {
		fi, err := os.Create(pinfile)
		if err != nil {
			return err
		}

		fi.Close()
	}
	return nil
}

func main() {
	ctx := context.Background()
	name := flag.String("name", "pinbot-test", "set pinbot's nickname")
	server := flag.String("server", "irc.freenode.net:6667", "set server to connect to")
	channel := flag.String("channel", "#pinbot-test", "set channel to join")
	pre := flag.String("prefix", "!", "prefix of command messages")
	gw := flag.String("gateway", "https://ipfs.io", "IPFS-to-HTTP gateway to use for success messages")
	username := flag.String("user", "", "Cluster API username")
	pw := flag.String("pw", "", "Cluster API pw")

	flag.Parse()

	prefix = *pre
	gateway = *gw
	msgs = make(chan msgWrap, 500)
	go messageQueueProcess()

	err := ensurePinLogExists()
	if err != nil {
		panic(err)
	}

	clusterpeers := loadHosts("clusterpeers")

	for _, h := range loadHosts("clusterpeers") {
		maddr, err := ma.NewMultiaddr(h)
		if err != nil {
			panic(err)
		}

		cfg := &cluster.Config{
			APIAddr:  maddr,
			Username: *username,
			Password: *pw,
		}
		client, err := cluster.NewDefaultClient(cfg)
		if err != nil {
			panic(err)
		}
		clusters = append(clusters, client)
		clusterAddrs = append(clusterAddrs, h)
		shs = append(shs, client.IPFS(ctx))
		shsUrls = append(
			shsUrls,
			fmt.Sprintf("http://127.0.0.1:%d", cluster.DefaultProxyPort),
		)
	}

	if len(clusterpeers) == 0 {
		for _, h := range loadHosts("hosts") {
			shs = append(shs, shell.NewShell(h))
			shsUrls = append(shsUrls, "http://"+h)
		}
	}

	if err := friends.Load(); err != nil {
		if os.IsNotExist(err) {
			friends = DefaultFriendsList
		} else {
			panic(err)
		}
	}
	fmt.Println("loaded", len(friends.friends), "friends")

	bot, err = newBot(*server, *name)
	if err != nil {
		panic(err)
	}

	connectToFreenodeIpfs(bot, *channel)
	fmt.Println("Connection lost! attempting to reconnect!")
	bot.Close()

	recontime := time.Second
	for {
		// Dont try to reconnect this time
		bot, err = newBot(*server, *name)
		if err != nil {
			fmt.Println("ERROR CONNECTING: ", err)
			time.Sleep(recontime)
			recontime += time.Second
			continue
		}
		recontime = time.Second

		connectToFreenodeIpfs(bot, *channel)
		fmt.Println("Connection lost! attempting to reconnect!")
		bot.Close()
	}
}

func newBot(server, name string) (bot *hb.Bot, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			err = errors.New("bot creation panicked")
		}
	}()
	bot, err = hb.NewBot(server, name)
	if err == nil {
		logHandler := log.LvlFilterHandler(log.LvlDebug, log.StdoutHandler)
		bot.Logger.SetHandler(logHandler)
		bot.PingTimeout = time.Hour * 24 * 7
	}
	return
}

func connectToFreenodeIpfs(con *hb.Bot, channel string) {
	// Run() panics badly sometimes
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()
	con.AddTrigger(pinTrigger)
	con.AddTrigger(unpinTrigger)
	con.AddTrigger(pinClusterTrigger)
	con.AddTrigger(unpinClusterTrigger)
	con.AddTrigger(statusClusterTrigger)
	con.AddTrigger(statusOngoingTrigger)
	con.AddTrigger(recoverClusterTrigger)
	con.AddTrigger(listTrigger)
	con.AddTrigger(befriendTrigger)
	con.AddTrigger(shunTrigger)
	con.AddTrigger(OmNomNom)
	con.AddTrigger(EatEverything)
	con.Channels = []string{channel}
	con.Run()

	// clears anything remaining in incoming
	for _ = range con.Incoming {
	}
}
