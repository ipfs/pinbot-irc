package main

import (
	"bufio"
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
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	hb "github.com/whyrusleeping/hellabot"
)

var prefix string
var gateway string

var (
	cmdBotsnack    = "botsnack"
	cmdFriends     = "friends"
	cmdBefriend    = "befriend"
	cmdShun        = "shun"
	cmdPin         = "pin"
	cmdUnPin       = "unpin"
	cmdStatus      = "status"
	cmdPinLegacy   = "legacypin"
	cmdUnpinLegacy = "legacyunpin"
)

var friends FriendsList

func init() {
	rand.Seed(123)
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

	b.Msg(actor, fmt.Sprintf("now pinning on %d nodes", len(shs)))

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
		b.Msg(actor, err.Error())
		failed++
	}

	successes := len(shs) - failed
	b.Msg(actor, fmt.Sprintf("pinned on %d of %d nodes (%d failures) -- %s%s",
		successes, len(shs), failed, gateway, path))

	if err := writePin(path, label); err != nil {
		b.Msg(actor, fmt.Sprintf("failed to write log entry for last pin: %s", err))
	}
	clusterPinUnpin(b, actor, path, label, true)
}

func Unpin(b *hb.Bot, actor, path string) {
	if !strings.HasPrefix(path, "/ipfs") && !strings.HasPrefix(path, "/ipns") {
		path = "/ipfs/" + path
	}

	errs := make(chan error, len(shs))
	var wg sync.WaitGroup

	b.Msg(actor, fmt.Sprintf("now unpinning on %d nodes", len(shs)))

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
		b.Msg(actor, err.Error())
		failed++
	}

	successes := len(shs) - failed
	b.Msg(actor, fmt.Sprintf("unpinned on %d of %d nodes (%d failures) -- %s%s",
		successes, len(shs), failed, gateway, path))
	clusterPinUnpin(b, actor, path, "", false)
}

func StatusCluster(b *hb.Bot, actor, path string) {
	// pick random cluster
	i := rand.Intn(len(clusters))
	cluster := clusters[i]
	addr := clusterAddrs[i]

	c, err := resolveCid(path, shs[i], shsUrls[i])
	if err != nil {
		b.Msg(actor, fmt.Sprintf("could not resolve cid: %s", err))
		return
	}

	st, err := cluster.Status(c, false)
	if err != nil {
		b.Msg(actor, fmt.Sprintf("%s: error obtaining pin status: %s", addr, err))
		return
	}
	prettyClusterStatus(b, actor, st)
}

func prettyClusterStatus(h *hb.Bot, actor string, st api.GlobalPinInfo) {
	h.Msg(actor, fmt.Sprintf("Cluster status for %s:", st.Cid))
	for peer, info := range st.PeerMap {
		h.Msg(actor, fmt.Sprintf("%s | %s | %s\n", peer.Pretty(), info.Status, info.Error))
	}
}

func PinCluster(b *hb.Bot, actor, path, label string) {
	clusterPinUnpin(b, actor, path, label, true)
	if err := writePin(path, label); err != nil {
		b.Msg(actor, fmt.Sprintf("failed to write log entry for last pin: %s", err))
	}
}

func UnpinCluster(b *hb.Bot, actor, path string) {
	clusterPinUnpin(b, actor, path, "", false)
}

func resolveCid(path string, sh *shell.Shell, url string) (*cid.Cid, error) {
	// fix path
	if !strings.HasPrefix(path, "/ipfs") && !strings.HasPrefix(path, "/ipns") {
		path = "/ipfs/" + path
	}

	// resolve it
	cidstr, err := sh.ResolvePath(path)
	if err != nil {
		return nil, err
	}

	return cid.Decode(cidstr)
}

func waitForClusterOp(b *hb.Bot, actor string, cluster *cluster.Client, c *cid.Cid, target api.TrackerStatus, rplFactor int) {
	b.Msg(actor, fmt.Sprintf("%s: waiting for status to reach %s", c, target))

	doneMap := make(map[peer.ID]bool)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Minute * 10)

	if rplFactor <= 0 {
		rplFactor = len(clusters)
	}

	for {
		time.Sleep(2 * time.Second)

		errors := false
		st, err := cluster.Status(c, false)
		if err != nil {
			b.Msg(actor, fmt.Sprintf("%s: failed to fetch status. Please retrigger the operation.", c))
			return
		}

		select {
		case <-ticker.C:
			b.Msg(actor, fmt.Sprintf("%s: so far pinned in %d/%d nodes", c, len(doneMap), rplFactor))
			continue
		case <-timeout.C:
			b.Msg(actor, fmt.Sprintf("%s: still pinning. Will not print any more notifications. Run !status to check", c))
			return
		default:
			for peer, info := range st.PeerMap {
				if info.Status == target {
					doneMap[peer] = true
				}

				if info.Error != "" {
					errors = true
					doneMap[peer] = false
				}
			}
		}

		if len(doneMap) == rplFactor {
			if !errors {
				b.Msg(actor, fmt.Sprintf("%s: reached %s in all %d required cluster peers", c, target, rplFactor))
				return
			}
			b.Msg(actor, fmt.Sprintf("%s: operation finished with errors. You will need to recover or retrigger the operation:", c))
			prettyClusterStatus(b, actor, st)

		}
	}
}

func clusterPinUnpin(b *hb.Bot, actor, path, label string, pin bool) {
	verb := "pin"
	if !pin {
		verb = "unpin"
	}

	// pick up a random cluster
	i := rand.Intn(len(clusters))
	cluster := clusters[i]
	addr := clusterAddrs[i]

	b.Msg(actor, fmt.Sprintf("Now cluster-%sning via %s", verb, addr))

	c, err := resolveCid(path, shs[i], shsUrls[i])
	if err != nil {
		b.Msg(actor, fmt.Sprintf("could not determine cid to pin: %s", err))
		return
	}

	b.Msg(actor, fmt.Sprintf("%s resolved as %s", path, c))

	switch pin {
	case true:
		err = cluster.Pin(c, 0, label)
		if err == nil {
			waitForClusterOp(b, actor, cluster, c, api.TrackerStatusPinned, 0)
		}
	case false:
		err = cluster.Unpin(c)
		if err == nil {
			waitForClusterOp(b, actor, cluster, c, api.TrackerStatusUnpinned, 0)
		}
	}

	if err != nil {
		b.Msg(actor, fmt.Sprintf("%s: failed to %s in cluster: %s", addr, verb, err))
		return
	}
}

var shs []*shell.Shell
var shsUrls []string
var clusters []*cluster.Client
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
	name := flag.String("name", "pinbot-test", "set pinbot's nickname")
	server := flag.String("server", "irc.freenode.net:6667", "set server to connect to")
	channel := flag.String("channel", "#pinbot-test", "set channel to join")
	pre := flag.String("prefix", "!", "prefix of command messages")
	gw := flag.String("gateway", "https://ipfs.io", "IPFS-to-HTTP gateway to use for success messages")
	flag.Parse()

	prefix = *pre
	gateway = *gw

	err := ensurePinLogExists()
	if err != nil {
		panic(err)
	}

	for _, h := range loadHosts("hosts") {
		shs = append(shs, shell.NewShell(h))
		shsUrls = append(shsUrls, "http://"+h)
	}

	for _, h := range loadHosts("clusterpeers") {
		maddr, err := ma.NewMultiaddr(h)
		if err != nil {
			panic(err)
		}

		cfg := &cluster.Config{APIAddr: maddr}
		client, err := cluster.NewClient(cfg)
		if err != nil {
			panic(err)
		}
		clusters = append(clusters, client)
		clusterAddrs = append(clusterAddrs, h)
	}

	if err := friends.Load(); err != nil {
		if os.IsNotExist(err) {
			friends = DefaultFriendsList
		} else {
			panic(err)
		}
	}
	fmt.Println("loaded", len(friends.friends), "friends")

	con, err := hb.NewBot(*server, *name, hb.ReconOpt())
	if err != nil {
		panic(err)
	}

	connectToFreenodeIpfs(con, *channel)
	fmt.Println("Connection lost! attempting to reconnect!")
	con.Close()

	recontime := time.Second
	for {
		// Dont try to reconnect this time
		con, err := hb.NewBot(*server, *name)
		if err != nil {
			fmt.Println("ERROR CONNECTING: ", err)
			time.Sleep(recontime)
			recontime += time.Second
			continue
		}
		recontime = time.Second

		connectToFreenodeIpfs(con, *channel)
		fmt.Println("Connection lost! attempting to reconnect!")
		con.Close()
	}
}

func connectToFreenodeIpfs(con *hb.Bot, channel string) {
	con.AddTrigger(pinTrigger)
	con.AddTrigger(unpinTrigger)
	con.AddTrigger(pinClusterTrigger)
	con.AddTrigger(unpinClusterTrigger)
	con.AddTrigger(statusClusterTrigger)
	con.AddTrigger(listTrigger)
	con.AddTrigger(befriendTrigger)
	con.AddTrigger(shunTrigger)
	con.AddTrigger(OmNomNom)
	con.AddTrigger(EatEverything)
	con.Channels = []string{channel}
	con.Run()

	for _ = range con.Incoming {
	}
}
