package disgoreact

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

type (
	// WatchContext contains the objects and tickrate needed to watch a message
	WatchContext struct {
		// Message is a DiscordGo Message object pointer
		Message *discordgo.Message
		// Session is a DiscordGo Session object pointer
		Session *discordgo.Session
		// TickRate is how frequently to poll the reactions on the message
		TickRate time.Duration
		// Data (for lack of a better name). An interface for storing just about anything
		Data interface{}
	}

	// Option contains a callback and expiration for a given emoji
	Option struct {
		// A unicode representation of the emoji option
		Emoji string
		// OnSucess is the function to call every time MinClicks has been met on the given emoji
		OnSucess func(user *discordgo.User, watchContext *WatchContext)
		// OnError is the function to call when the watcher or poller encounters an error
		OnError func(err error, watchContext *WatchContext)
		// ReactionLimit is the cap for the total number of reactions polled
		ReactionLimit int
		// Expiration as a Timer
		Expiration time.Duration
	}
)

// NewWatcher creates a new WatchContext
func NewWatcher(
	message *discordgo.Message,
	session *discordgo.Session,
	tickRate time.Duration,
	data interface{},
) (*WatchContext, error) {
	if tickRate == 0 {
		return &WatchContext{}, fmt.Errorf("no tickrate specified (cannot be 0)")
	}

	return &WatchContext{
		Message:  message,
		Session:  session,
		TickRate: tickRate,
		Data:     data,
	}, nil
}

// Add adds watchers to the given WatchContext corresponding to the given
// reaction. Reactions are specified as an array of Options.
func (ctx *WatchContext) Add(options ...Option) error {
	if len(options) == 0 {
		return fmt.Errorf("no emoji options specified")
	}

	/* Iterate through options and add corresponding reactions and handlers */
	for _, v := range options {
		err := ctx.Session.MessageReactionAdd(
			ctx.Message.ChannelID, ctx.Message.ID, v.Emoji,
		)
		if err != nil {
			return fmt.Errorf(
				"can't add reaction to message %q. Was that a unicode emoji?",
				ctx.Message.ID,
			)
		}

		/* Fire up watcher */
		go ctx.watcher(v)
	}

	return nil
}

func (ctx *WatchContext) watcher(opt Option) {
	expiration := time.After(opt.Expiration)
	tick := time.Tick(ctx.TickRate)
	expired := false

	for {
		/* Check expiration timer. If expired or if stop requested, stop */
		select {
		case <-expiration:
			expired = true
		case <-tick:
			if expired {
				ctx.Session.MessageReactionsRemoveAll(
					ctx.Message.ChannelID, ctx.Message.ID,
				)
				return
			}

			/* Poll the message. If there is a new reaction (i.e. total reactions > 1) return a user */
			user, err := poll(ctx.Session, ctx.Message.ChannelID, ctx.Message.ID, &opt)
			if err != nil {
				opt.OnError(err, ctx)
				return
			}

			/* Make sure we're actually returning a user */
			if (discordgo.User{}) != *user {
				opt.OnSucess(user, ctx)
			}
		}
	}
}

func poll(ses *discordgo.Session, chID, msID string, opt *Option) (*discordgo.User, error) {
	users, err := ses.MessageReactions(
		chID, msID, opt.Emoji,
		opt.ReactionLimit,
	)
	if err != nil {
		return &discordgo.User{}, err
	}

	/* If there is more than one reaction (the bot's reaction is one of them) */
	if len(users) >= 1 {
		/* Iterate through the users, ignore the bot, remove reaction, return user */
		for _, u := range users {
			if u.ID == ses.State.User.ID {
				continue
			}

			err := ses.MessageReactionRemove(chID, msID, opt.Emoji, u.ID)
			if err != nil {
				return &discordgo.User{}, err
			}
			return u, nil
		}

	}
	return &discordgo.User{}, nil
}
