package loafer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// SlackApp - A simple slack app starter kit
type SlackApp struct {
	opts              SlackAppOptions
	server            *http.Server                                                                           // Slack App options
	distCB            func(installRes *SlackOauth2Response, res http.ResponseWriter, req *http.Request) bool // Handler for app distribution
	cmds              map[string]func(ctx *SlackContext)                                                     // List of command handlers
	shortcutListeners map[string]func(ctx *SlackContext)                                                     // List of shortcut handlers
	actionListeners   map[string]func(ctx *SlackContext)                                                     // List of action handlers
	submitListeners   map[string]func(ctx *SlackContext)                                                     // List of view submission handlers
	closeListeners    map[string]func(ctx *SlackContext)                                                     // List of view close handlers
}

// SlackAuthToken - Slack App Auth Token
type SlackAuthToken struct {
	Workspace string
	Token     string
}

// SlackAppOptions - Slack App options
type SlackAppOptions struct {
	Name          string           // Slack App name
	Prefix        string           // Prefix of routes
	Tokens        []SlackAuthToken // List of available workspace tokens
	ClientSecret  string           // App client secret
	ClientID      string           // App client id
	SigningSecret string           // Signning secret
}

// SlackContext - Slack request context
type SlackContext struct {
	Body  []byte
	Token string
	Req   *http.Request
	Res   http.ResponseWriter
}

// SlackOauth2Team - Slack App Access Response Team
type SlackOauth2Team struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// SlackOauth2User - Slack App Access Response User
type SlackOauth2User struct {
	ID          string `json:"id"`
	Scope       string `json:"scope"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// SlackOauth2Response - Slack App Access Response
type SlackOauth2Response struct {
	Ok          bool            `json:"ok"`
	AccessToken string          `json:"access_token"`
	TokenType   string          `json:"token_type"`
	Scope       string          `json:"scope"`
	BotUserID   string          `json:"bot_user_id"`
	AppID       string          `json:"app_id"`
	Team        SlackOauth2Team `json:"team"`
	Enterprise  SlackOauth2Team `json:"enterprise"`
	AuthedUser  SlackOauth2User `json:"authed_user"`
}

// SetTokens - Set token list
func (a *SlackApp) SetTokens(tokens []SlackAuthToken) {
	a.opts.Tokens = tokens
}

// AddToken - Add token to list
func (a *SlackApp) AddToken(token SlackAuthToken) {
	a.opts.Tokens = append(a.opts.Tokens, token)
}

// OnCommand - Add handler to command
func (a *SlackApp) OnCommand(cmd string, handler func(ctx *SlackContext)) {
	if a.cmds == nil {
		a.cmds = make(map[string]func(ctx *SlackContext))
	}
	a.cmds[cmd] = handler
}

// RemoveCommand - Remove a command to the app base on command
func (a *SlackApp) RemoveCommand(cmd string) {
	if a.cmds != nil {
		delete(a.cmds, cmd)
	}
}

// OnAction - Add an action handler to the app base on action_id
func (a *SlackApp) OnAction(actionID string, handler func(ctx *SlackContext)) {
	if a.actionListeners == nil {
		a.actionListeners = make(map[string]func(ctx *SlackContext))
	}
	a.actionListeners[actionID] = handler
}

// OnShortcut - Add an shortcut handler to the app base on callback_id
func (a *SlackApp) OnShortcut(callbackID string, handler func(ctx *SlackContext)) {
	if a.shortcutListeners == nil {
		a.shortcutListeners = make(map[string]func(ctx *SlackContext))
	}
	a.shortcutListeners[callbackID] = handler
}

// OnViewSubmission - Add handler to view submission base on callback_id
func (a *SlackApp) OnViewSubmission(callbackID string, handler func(ctx *SlackContext)) {
	if a.submitListeners == nil {
		a.submitListeners = make(map[string]func(ctx *SlackContext))
	}
	a.submitListeners[callbackID] = handler
}

// OnViewClose - Add handler to view close base on callback_id
func (a *SlackApp) OnViewClose(callbackID string, handler func(ctx *SlackContext)) {
	if a.closeListeners == nil {
		a.closeListeners = make(map[string]func(ctx *SlackContext))
	}
	a.closeListeners[callbackID] = handler
}

// OnAppInstall - Add handler to app distribution after it's been successfully installed
func (a *SlackApp) OnAppInstall(cb func(installRes *SlackOauth2Response, res http.ResponseWriter, req *http.Request) bool) {
	a.distCB = cb
}

// appInstall - Handler for app distribution
func (a *SlackApp) appInstall(res http.ResponseWriter, req *http.Request) {
	var installResponse SlackOauth2Response
	form := url.Values{}
	form.Set("code", req.URL.Query().Get("code"))
	form.Set("client_id", a.opts.ClientID)
	form.Set("client_secret", a.opts.ClientSecret)
	resp, err := http.Post("https://slack.com/api/oauth.v2.access", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		Response(&SlackContext{Res: res}, http.StatusInternalServerError, []byte("Unable to authorize Slack App for workspace"), nil)
		return
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&installResponse)
	if err != nil {
		Response(&SlackContext{Res: res}, http.StatusInternalServerError, []byte("Unable to get Slack OAuth2 Access Response for workspace"), nil)
		return
	}
	if installResponse.Ok {
		avoidDefaultPage := false
		if a.distCB != nil {
			a.AddToken(SlackAuthToken{
				Workspace: installResponse.Team.ID,
				Token:     installResponse.AccessToken})
			avoidDefaultPage = a.distCB(&installResponse, res, req)
		}
		if !avoidDefaultPage {
			Response(&SlackContext{Res: res}, http.StatusOK, []byte(strings.Replace(INSTALLSUCCESSPAGE, "{{APP_NAME}}", a.opts.Name, -1)), map[string]string{
				"Content-Type": "text/html; charset=utf-8"})
			return
		}
	} else {
		Response(&SlackContext{Res: res}, http.StatusInternalServerError, []byte("Slack App Access Request is not Ok"), nil)
		return
	}
}

// checkSlackSecret - Checking the signing secret of slack request
func (a *SlackApp) checkSlackSecret(signing string, ts string, body string) bool {
	data := strings.Join([]string{"v0", ts, body}, ":")
	signed := []byte(a.opts.SigningSecret)
	tested := hmac.New(sha256.New, []byte(signed))
	tested.Write([]byte(data))
	own := strings.Join([]string{"v0", hex.EncodeToString(tested.Sum(nil))}, "=")
	if own == signing {
		return true
	}
	return false
}

// interaction - Slack App interactions handler
func (a *SlackApp) interactions(res http.ResponseWriter, req *http.Request) {
	var event SlackInteractionEvent
	bodyText, err := ioutil.ReadAll(req.Body)
	if err != nil {
		Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("Invalid Body"), nil)
		return
	}
	defer req.Body.Close()
	queries, err := url.ParseQuery(string(bodyText))
	if err != nil {
		Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("Invalid Form Body"), nil)
		return
	}
	isAuthorizedCaller := a.checkSlackSecret(req.Header.Get("X-Slack-Signature"), req.Header.Get("X-Slack-Request-TimeStamp"), string(bodyText))
	if isAuthorizedCaller {
		err = json.Unmarshal([]byte(queries.Get("payload")), &event)
		if err != nil {
			Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("Invalid JSON format"), nil)
			return
		}
		accessToken := findTokenForWorkspace(&a.opts.Tokens, event.Team.ID)
		if accessToken == nil {
			fmt.Printf("App not installed for workspace: %s\n", queries.Get("team_id"))
			Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("App not installed for workspace"), nil)
			return
		}
		ctx := &SlackContext{
			Body:  bodyText,
			Token: accessToken.Token,
			Res:   res,
			Req:   req}
		switch Type := event.Type; Type {
		case "shortcut":
			callbackID := event.CallbackID
			if handler, ok := a.shortcutListeners[callbackID]; ok {
				handler(ctx)
			} else {
				fmt.Printf("Unrecognized shortcut: %s\n", callbackID)
				Response(ctx, http.StatusBadRequest, []byte("Unrecognized shortcut callback_id"), nil)
				return
			}
		case "block_actions":
			action := event.Actions[0]
			if handler, ok := a.actionListeners[action.ActionID]; ok {
				handler(ctx)
			} else {
				fmt.Printf("Unrecognized action: %s\n", action.ActionID)
				Response(ctx, http.StatusBadRequest, []byte("Unrecognized action action_id"), nil)
				return
			}
			break
		case "view_submission":
			if handler, ok := a.submitListeners[event.View.CallbackID]; ok {
				handler(ctx)
			} else {
				fmt.Printf("Unrecognized submission event from view: %s\n", event.View.CallbackID)
				Response(ctx, http.StatusBadRequest, []byte("Unrecognized view submission callback_id"), nil)
				return
			}
		case "view_closed":
			if handler, ok := a.closeListeners[event.View.CallbackID]; ok {
				handler(ctx)
			} else {
				fmt.Printf("Unrecognized closed event from view: %s\n", event.View.CallbackID)
				Response(ctx, http.StatusBadRequest, []byte("Unrecognized view closed callback_id"), nil)
				return
			}
		default:
			Response(ctx, http.StatusBadRequest, []byte("Unrecognized interaction type"), nil)
		}
	} else {
		Response(&SlackContext{Res: res}, http.StatusUnauthorized, []byte("Unauthorized"), nil)
		return
	}
}

// commands - Slack App commands handler
func (a *SlackApp) commands(res http.ResponseWriter, req *http.Request) {
	bodyText, err := ioutil.ReadAll(req.Body)
	if err != nil {
		Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("Invalid Body"), nil)
		return
	}
	defer req.Body.Close()
	queries, err := url.ParseQuery(string(bodyText))
	if err != nil {
		Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("Invalid Form Body"), nil)
		return
	}
	isAuthorizedCaller := a.checkSlackSecret(req.Header.Get("X-Slack-Signature"), req.Header.Get("X-Slack-Request-TimeStamp"), string(bodyText))
	if isAuthorizedCaller {
		accessToken := findTokenForWorkspace(&a.opts.Tokens, queries.Get("team_id"))
		if accessToken == nil {
			fmt.Printf("App not installed for workspace: %s\n", queries.Get("team_id"))
			Response(&SlackContext{Res: res}, http.StatusBadRequest, []byte("App not installed for workspace"), nil)
			return
		}
		ctx := &SlackContext{
			Body:  bodyText,
			Token: accessToken.Token,
			Res:   res,
			Req:   req}
		if accessToken == nil {
			Response(ctx, http.StatusBadRequest, []byte("Unrecognized workspace"), nil)
			return
		}
		if handler, ok := a.cmds[queries.Get("command")]; ok {
			handler(ctx)
		} else {
			fmt.Printf("Unrecognized command: %s\n", queries.Get("command"))
			Response(ctx, http.StatusBadRequest, []byte("Unrecognized command"), nil)
			return
		}
	} else {
		Response(&SlackContext{Res: res}, http.StatusUnauthorized, []byte("Unauthorized"), nil)
		return
	}
}

func (a *SlackApp) index(res http.ResponseWriter, req *http.Request) {
	Response(&SlackContext{Res: res}, http.StatusOK, nil, nil)
}

// ServeApp - Listen and Serve App on desired port, callback can be nil
func (a *SlackApp) ServeApp(port uint16, cb func()) {
	if len(a.opts.Prefix) == 0 {
		panic(fmt.Sprintf("\x1b[31m%s\x1b[0m\n", "Slack App Route Prefix Cannot Be Empty"))
	}
	a.server = &http.Server{Addr: fmt.Sprintf(":%d", port)}
	http.HandleFunc("/", a.index)
	http.HandleFunc(fmt.Sprintf("/%s/install", a.opts.Prefix), a.appInstall)
	http.HandleFunc(fmt.Sprintf("/%s/commands", a.opts.Prefix), a.commands)
	http.HandleFunc(fmt.Sprintf("/%s/", a.opts.Prefix), a.interactions)
	if cb != nil {
		go cb()
	}
	a.server.ListenAndServe()
}

// Close - Shutting down the server
func (a *SlackApp) Close(ctx context.Context) {
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			panic(err)
		}
	}
}

// InitializeSlackApp - Return an instance of SlackApp
func InitializeSlackApp(opts *SlackAppOptions) SlackApp {
	app := SlackApp{
		opts: SlackAppOptions{
			Name:          opts.Name,
			Tokens:        opts.Tokens,
			Prefix:        opts.Prefix,
			ClientSecret:  opts.ClientSecret,
			ClientID:      opts.ClientID,
			SigningSecret: opts.SigningSecret},
		distCB:          nil,
		cmds:            make(map[string]func(ctx *SlackContext)),
		actionListeners: make(map[string]func(ctx *SlackContext)),
		submitListeners: make(map[string]func(ctx *SlackContext)),
		closeListeners:  make(map[string]func(ctx *SlackContext)),
	}
	return app
}

// findTokenForWorkspace - Finding the token for the corresponding workspace
func findTokenForWorkspace(tokens *[]SlackAuthToken, workspace string) *SlackAuthToken {
	var token *SlackAuthToken
	for _, t := range *tokens {
		if t.Workspace == workspace {
			token = &t
			break
		}
	}
	return token
}

// Response - Send response back to slack
func Response(ctx *SlackContext, code int, message []byte, headers map[string]string) {
	for k, v := range headers {
		ctx.Res.Header().Set(k, v)
	}
	ctx.Res.WriteHeader(code)
	ctx.Res.Write(message)
}

// ConvertState - Convert unknown state to struct
func ConvertState(state ISlackBlockKitUI, dst interface{}) error {
	jsonView, err := json.Marshal(state)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonView, dst)
	if err != nil {
		return err
	}
	return nil
}
