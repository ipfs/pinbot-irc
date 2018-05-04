package main

import (
	"strings"

	hb "github.com/whyrusleeping/hellabot"
)

var EatEverything = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return true
	},
	func(irc *hb.Bot, mes *hb.Message) bool {
		//fmt.Println(mes)
		return true
	},
}

var OmNomNom = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return mes.Content == prefix+cmdBotsnack
	},
	func(irc *hb.Bot, mes *hb.Message) bool {
		irc.Msg(mes.To, "om nom nom")
		return true
	},
}

var authTrigger = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return true
	},
	func(con *hb.Bot, mes *hb.Message) bool {
		if friends.CanPin(mes.From) {
			// do not consume messages from authed users
			return false
		}
		return true
	},
}

var pinTrigger = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdPinLegacy)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
		cmd := strings.TrimPrefix(mes.Content, prefix)
		parts := strings.Fields(cmd)
		if len(parts) < 3 {
			con.Msg(mes.To, "usage: !pin <hash> <label>")
		} else {
			Pin(con, mes.To, parts[1], strings.Join(parts[2:], " ", mes.From))
		}
		return true
	},
}

var unpinTrigger = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdUnpinLegacy)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
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
	func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdPin)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
		cmd := strings.TrimPrefix(mes.Content, prefix)
		parts := strings.Fields(cmd)
		if len(parts) < 3 {
			con.Msg(mes.To, "usage: !pinc <hash> <label>")
		} else {
			PinCluster(con, mes.To, parts[1], strings.Join(parts[2:], " "))
		}
		return true
	},
}

var unpinClusterTrigger = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanPin(mes.From) && strings.HasPrefix(mes.Content, prefix+cmdUnPin)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
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
	func(irc *hb.Bot, mes *hb.Message) bool {
		return strings.HasPrefix(mes.Content, prefix+cmdStatus)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
		parts := strings.Split(mes.Content, " ")
		if len(parts) == 1 {
			con.Msg(mes.To, "usage: !status <hash>")
		} else {
			StatusCluster(con, mes.To, parts[1])
		}
		return true
	},
}

var listTrigger = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return mes.Content == prefix+cmdFriends
	},
	func(con *hb.Bot, mes *hb.Message) bool {
		out := "my friends are: "
		for n, _ := range friends.friends {
			out += n + " "
		}
		con.Notice(mes.From, out)
		return true
	},
}

var befriendTrigger = hb.Trigger{
	func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanAddFriends(mes.From) &&
			strings.HasPrefix(mes.Content, prefix+cmdBefriend)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
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
	func(irc *hb.Bot, mes *hb.Message) bool {
		return friends.CanAddFriends(mes.From) &&
			strings.HasPrefix(mes.Content, prefix+cmdShun)
	},
	func(con *hb.Bot, mes *hb.Message) bool {
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
