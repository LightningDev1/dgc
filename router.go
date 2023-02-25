package dgc

import (
	"regexp"
	"strings"

	"github.com/LightningDev1/discordgo"
)

// regexSplitting represents the regex to split the arguments at
var regexSplitting = regexp.MustCompile("\\s+")

// Router represents a DiscordGo command router
type Router struct {
	Prefixes         []string
	PrefixFunc       func() []string
	IgnorePrefixCase bool
	BotsAllowed      bool
	SelfBot          bool
	IsUserAllowedFunc func(*Ctx) bool
	Commands         []*Command
	Middlewares      []Middleware
	Categories       []*Category
	PingHandler      ExecutionHandler
	Storage          map[string]*ObjectsMap
	currentCategory  string
}

// Create makes sure all maps get initialized
func Create(router *Router) *Router {
	router.Storage = make(map[string]*ObjectsMap)
	router.InitializeStorage("categories")
	return router
}

// StartCategory starts a new category
// All commands registered after a call to StartCategory will be in the given category
func (router *Router) StartCategory(name string, description string) {
	router.Storage["categories"].Set(name, []*Command{})
	router.Categories = append(router.Categories, &Category{
		Name:        name,
		Description: description,
	})
	router.currentCategory = name
}

// StopCategory stops the current category
func (router *Router) StopCategory() {
	router.currentCategory = ""
}

// GetCategory returns the category with the given name if it exists
func (router *Router) GetCategory(name string) *Category {
	var category *Category
	for _, category_ := range router.Categories {
		if strings.EqualFold(category_.Name, name) {
			category = category_
		}
	}
	if category != nil {
		if categories, ok := router.Storage["categories"]; ok {
			commands, success := categories.Get(name)
			if success {
				category.Commands = commands.([]*Command)
				return category
			}
		}
	}
	return nil
}

// RegisterCmd registers a new command
func (router *Router) RegisterCmd(command *Command) {
	if router.currentCategory != "" {
		category := router.Storage["categories"]
		if category != nil {
			categoryCommands, success := category.Get(router.currentCategory)
			if success {
				categoryCommands = append(categoryCommands.([]*Command), command)
				category.Set(router.currentCategory, categoryCommands)
				command.Category = router.GetCategory(router.currentCategory)
			}
		}
	}
	router.Commands = append(router.Commands, command)
}

// GetCmd returns the command with the given name if it exists
func (router *Router) GetCmd(name string) *Command {
	// Loop through all commands to find the correct one
	for _, command := range router.Commands {
		// Define the slice to check
		toCheck := make([]string, len(command.Aliases)+1)
		toCheck = append(toCheck, command.Name)
		toCheck = append(toCheck, command.Aliases...)

		// Check the prefix of the string
		if stringArrayContains(toCheck, name, command.IgnoreCase) {
			return command
		}
	}
	return nil
}

// RegisterMiddleware registers a new middleware
func (router *Router) RegisterMiddleware(middleware Middleware) {
	router.Middlewares = append(router.Middlewares, middleware)
}

// InitializeStorage initializes a storage map
func (router *Router) InitializeStorage(name string) {
	router.Storage[name] = newObjectsMap()
}

// Initialize initializes the message event listener
func (router *Router) Initialize(session *discordgo.Session) {
	session.AddHandler(router.Handler())
}

func (router *Router) GetPrefixes() []string {
	prefixes := router.Prefixes
	if router.PrefixFunc != nil {
		prefixes = router.PrefixFunc()
	}
	return prefixes
}

// Handler provides the discordgo handler for the given router
func (router *Router) Handler() func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(session *discordgo.Session, event *discordgo.MessageCreate) {
		// Define useful variables
		message := event.Message
		content := message.Content

		// Check if the message was sent by a bot
		if message.Author.Bot && !router.BotsAllowed {
			return
		}

		// Execute the ping handler if the message equals the current bot's mention
		if (content == "<@!"+session.State.User.ID+">" || content == "<@"+session.State.User.ID+">") && router.PingHandler != nil {
			router.PingHandler(&Ctx{
				Session:   session,
				Event:     event,
				Arguments: ParseArguments(""),
				Router:    router,
			})
			return
		}

		// Check if the message starts with one of the defined prefixes
		prefixes := router.GetPrefixes()
		hasPrefix, content := stringHasPrefix(content, prefixes, router.IgnorePrefixCase)
		if !hasPrefix {
			return
		}

		// Get rid of additional spaces
		content = strings.Trim(content, " ")

		// Check if the message is empty after the prefix processing
		if content == "" {
			return
		}

		// Split the messages at any whitespace
		parts := regexSplitting.Split(content, -1)

		if parts[0] == "" {
			return
		}

		// Check if the message starts with a command name
		for _, command := range router.Commands {
			// Check if the first part is the current command
			if !stringArrayContains(getIdentifiers(command), parts[0], command.IgnoreCase) {
				continue
			}
			content = strings.Join(parts[1:], " ")

			// Define the command context
			ctx := &Ctx{
				Session:       session,
				Event:         event,
				Arguments:     ParseArguments(content),
				CustomObjects: newObjectsMap(),
				Router:        router,
				Command:       command,
			}

			// Check if the user is allowed to use the bot
			if router.IsUserAllowedFunc != nil {
				if !router.IsUserAllowedFunc(ctx) {
					return
				}
			}

			// Trigger the command
			command.trigger(ctx)
		}
	}
}

func getIdentifiers(command *Command) []string {
	// Define an array containing the commands name and the aliases
	toCheck := make([]string, len(command.Aliases)+1)
	toCheck = append(toCheck, command.Name)
	toCheck = append(toCheck, command.Aliases...)
	return toCheck
}
