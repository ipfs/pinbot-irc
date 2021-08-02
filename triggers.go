package main

import (
	"strings"

	"github.com/ipfs/ipfs-cluster/api"

	hb "github.com/whyrusleeping/hellabot"
)

var EatEverything = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return true
	},
	Action: func(irc *hb.Bot, mes *hb.Message) bool {
		//fmt.Println(mes)
		return true
	},
}

var OmNomNom = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return mes.Content == prefix+cmdBotsnack
	},
	Action: func(irc *hb.Bot, mes *hb.Message) bool {
		irc.Msg(mes.To, "om nom nom")
		return true
	},
}

var authTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return true
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		// do not consume messages from authed users
		return !friends.CanPin(mes.From)
	},
}

var pinTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdPinLegacy)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		cmd := strings.TrimPrefix(mes.Content, prefix)
		parts := strings.Fields(cmd)
		if len(parts) < 3 {
			con.Msg(mes.To, "usage: !pin <hash> <label>")
		} else {
			Pin(con, mes.To, parts[1], strings.Join(parts[2:], " "))
		}
		return true
	},
}

var unpinTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdUnpinLegacy)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		parts := strings.Split(mes.Content, " ")
		if len(parts) == 1 {
			con.Msg(mes.To, "what do you want me to unpin?")
		} else {
			Unpin(con, mes.To, parts[1])
		}
		return true
	},
}

var pinClusterTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdPin)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		cmd := strings.TrimPrefix(mes.Content, prefix)
		parts := strings.Fields(cmd)
		if len(parts) < 3 {
			con.Msg(mes.To, "usage: !pin <hash> <label>")
		} else {
			PinCluster(con, mes.To, parts[1], strings.Join(parts[2:], " "))
		}
		return true
	},
}

var unpinClusterTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdUnPin)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		parts := strings.Split(mes.Content, " ")
		if len(parts) == 1 {
			con.Msg(mes.To, "what do you want me to unpin from cluster?")
		} else {
			UnpinCluster(con, mes.To, parts[1])
		}
		return true
	},
}

var statusClusterTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return strings.HasPrefix(mes.Content, prefix+cmdStatus)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		parts := strings.Split(mes.Content, " ")
		if len(parts) == 1 {
			con.Msg(mes.To, "usage: !status <hash>")
		} else {
			StatusCluster(con, mes.To, parts[1])
		}
		return true
	},
}

var recoverClusterTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdRecover)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		cmd := strings.TrimPrefix(mes.Content, prefix)
		parts := strings.Fields(cmd)
		if len(parts) == 1 {
			con.Msg(mes.To, "usage: !recover <hash>")
		} else {
			RecoverCluster(con, mes.To, parts[1])
		}
		return true
	},
}

var statusOngoingTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return strings.HasPrefix(mes.Content, prefix+cmdOngoing)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		StatusAllCluster(con, mes.To, api.TrackerStatusError|api.TrackerStatusPinning|api.TrackerStatusQueued|api.TrackerStatusUnpinning)
		return true
	},
}

var listTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return mes.Content == prefix+cmdFriends
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		out := "my friends are: "
		for n := range friends.friends {
			out += n + " "
		}
		con.Notice(mes.From, out)
		return true
	},
}

var befriendTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanAddFriends(mes.From) &&
			strings.HasPrefix(mes.Content, prefix+cmdBefriend)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		parts := strings.Split(mes.Content, " ")
		if len(parts) != 3 {
			con.Msg(mes.To, prefix+cmdBefriend+" <name> <perm>")
			return true
		}
		name := parts[1]
		perm := parts[2]

		if err := friends.AddFriend(name, perm); err != nil {
			con.Msg(mes.To, "failed to befriend: "+err.Error())
			return true
		}
		con.Msg(mes.To, "Hey "+name+", let's be friends! You can "+perm)
		return true
	},
}

var shunTrigger = hb.Trigger{
	Condition: func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanAddFriends(mes.From) &&
			strings.HasPrefix(mes.Content, prefix+cmdShun)
	},
	Action: func(con *hb.Bot, mes *hb.Message) bool {
		parts := strings.Split(mes.Content, " ")
		if len(parts) != 2 {
			con.Msg(mes.To, "who do you want me to shun?")
			return true
		}

		name := parts[1]
		if err := friends.RmFriend(name); err != nil {
			con.Msg(mes.To, "failed to shun: "+err.Error())
			return true
		}
		con.Msg(mes.To, "shun "+name+" the non believer! Shuuuuuuuun")
		return true
	},
}
