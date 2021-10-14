// Copyright (c) 2021 InfraCloud Technologies
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package bot

import (
	"fmt"
	"github.com/infracloudio/botkube/pkg/config"
	"github.com/infracloudio/botkube/pkg/execute"
	"github.com/infracloudio/botkube/pkg/log"
	"github.com/infracloudio/botkube/pkg/utils"
	"github.com/larksuite/oapi-sdk-go/core"
	"github.com/larksuite/oapi-sdk-go/core/tools"
	"github.com/larksuite/oapi-sdk-go/event"
	eventhttpserver "github.com/larksuite/oapi-sdk-go/event/http/native"
	"net/http"
	"strings"
)

// LarkBot listens for user's message, execute commands and sends back the response
type LarkBot struct {
	AllowKubectl     bool
	RestrictAccess   bool
	ClusterName      string
	DefaultNamespace string
	Port             int
	MessagePath      string
	LarkClient       *utils.LarkClient
}

// NewLarkBot returns new Bot object
func NewLarkBot(c *config.Config) Bot {
	larkConf := c.Communications.Lark
	appSettings := core.NewInternalAppSettings(core.SetAppCredentials(larkConf.AppID, larkConf.AppSecret),
		core.SetAppEventKey(larkConf.VerificationToken, larkConf.EncryptKey))
	conf := core.NewConfig(core.Domain(larkConf.Endpoint), appSettings, core.SetLoggerLevel(core.LoggerLevelError))
	return &LarkBot{
		AllowKubectl:     c.Settings.Kubectl.Enabled,
		RestrictAccess:   c.Settings.Kubectl.RestrictAccess,
		ClusterName:      c.Settings.ClusterName,
		DefaultNamespace: c.Settings.Kubectl.DefaultNamespace,
		Port:             c.Communications.Lark.Port,
		MessagePath:      c.Communications.Lark.MessagePath,
		LarkClient:       utils.NewLarkClient(conf),
	}
}

// Execute execute commands sent by users
func (l *LarkBot) Execute(e map[string]interface{}) {
	event := e["event"].(map[string]interface{})

	chatType := event["chat_type"].(string)
	text := event["text_without_at_bot"].(string)

	executor := execute.NewDefaultExecutor(text, l.AllowKubectl, l.RestrictAccess, l.DefaultNamespace,
		l.ClusterName, config.LarkBot, "", true)
	response := executor.Execute()

	if chatType == "group" {
		l.LarkClient.SendTextMessage("chat_id", event["open_chat_id"].(string), response)
	} else {
		l.LarkClient.SendTextMessage("open_id", event["open_id"].(string), response)
	}
}

// Start starts the lark server and listens for lark messages
func (l *LarkBot) Start() {
	// See: https://open.larksuite.com/document/ukTMukTMukTM/ukjNxYjL5YTM24SO2EjN
	// message
	larkConf := l.LarkClient.Conf
	event.SetTypeCallback(larkConf, "message", func(ctx *core.Context, e map[string]interface{}) error {
		log.Infof(ctx.GetRequestID())
		log.Infof(tools.Prettify(e))
		go l.Execute(e)
		return nil
	})

	// add_bot
	event.SetTypeCallback(larkConf, "add_bot", func(ctx *core.Context, e map[string]interface{}) error {
		log.Infof(ctx.GetRequestID())
		log.Infof(tools.Prettify(e))
		go l.SayHello(e)
		return nil
	})

	// p2p_chat_create
	event.SetTypeCallback(larkConf, "p2p_chat_create", func(ctx *core.Context, e map[string]interface{}) error {
		log.Infof(ctx.GetRequestID())
		log.Infof(tools.Prettify(e))
		go l.SayHello(e)
		return nil
	})

	// add_user_to_chat
	event.SetTypeCallback(larkConf, "add_user_to_chat", func(ctx *core.Context, e map[string]interface{}) error {
		log.Infof(ctx.GetRequestID())
		log.Infof(tools.Prettify(e))
		go l.SayHello(e)
		return nil
	})

	eventhttpserver.Register(l.MessagePath, larkConf)
	log.Infof("Started lark server on port %d", l.Port)
	log.Errorf("Error in lark server. %v", http.ListenAndServe(fmt.Sprintf(":%d", l.Port), nil))
}

// SayHello send welcome message to new added users
func (l *LarkBot) SayHello(e map[string]interface{}) error {
	event := e["event"].(map[string]interface{})
	users := event["users"].([]interface{})
	var messages []string
	if users != nil {
		for _, user := range users {
			openID := user.(map[string]interface{})["open_id"].(string)
			username := user.(map[string]interface{})["user_id"].(string)
			messages = append(messages, fmt.Sprintf("<at user_id=\"%s\">%s</at>", openID, username))
		}
	}
	messages = append(messages, "Hello from botkube~ Play with me by at botkube <commands>")
	return l.LarkClient.SendTextMessage("chat_id", event["chat_id"].(string), strings.Join(messages, " "))
}
