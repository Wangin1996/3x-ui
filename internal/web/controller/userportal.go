package controller

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// clientLoginLimiter throttles end-user portal logins independently of the
// admin limiter, so a flood of client attempts never affects admin logins.
var clientLoginLimiter = newLoginLimiter(loginLimitMaxFailures, loginLimitWindow, loginLimitCooldown)

type clientLoginForm struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// ClientSelfView is the read-only self-service payload for a logged-in end user.
type ClientSelfView struct {
	Email         string   `json:"email"`
	Enable        bool     `json:"enable"`
	Up            int64    `json:"up"`
	Down          int64    `json:"down"`
	Total         int64    `json:"total"`
	ExpiryTime    int64    `json:"expiryTime"`
	LastOnline    int64    `json:"lastOnline"`
	SubID         string   `json:"subId"`
	SubURL        string   `json:"subUrl"`
	SubJsonURL    string   `json:"subJsonUrl"`
	SubClashURL   string   `json:"subClashUrl"`
	Links         []string `json:"links"`
	ExternalLinks any      `json:"externalLinks"`
	Ips           any      `json:"ips"`
}

// UserPortalController serves the end-user (proxy client) self-service portal:
// email+password login, a read-only usage/subscription view, and subscription
// token rotation. It uses a session key distinct from the admin's, so an
// end-user session can never reach /panel.
type UserPortalController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	settingService service.SettingService
}

func NewUserPortalController(g *gin.RouterGroup) *UserPortalController {
	a := &UserPortalController{}
	a.initRouter(g)
	return a
}

func (a *UserPortalController) initRouter(g *gin.RouterGroup) {
	g.GET("/user", a.page)
	g.POST("/user/login", middleware.CSRFMiddleware(), a.login)
	g.POST("/user/logout", middleware.CSRFMiddleware(), a.logout)
	g.GET("/user/api/me", a.me)
	g.POST("/user/api/rotateSub", middleware.CSRFMiddleware(), a.rotateSub)
}

func (a *UserPortalController) page(c *gin.Context) {
	serveDistPage(c, "user.html")
}

func (a *UserPortalController) login(c *gin.Context) {
	var form clientLoginForm
	if err := c.ShouldBind(&form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.user.toasts.wrongCredentials"))
		return
	}
	email := strings.TrimSpace(form.Email)
	remoteIP := getRemoteIp(c)
	if _, ok := clientLoginLimiter.allow(remoteIP, email); !ok {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.user.toasts.wrongCredentials"))
		return
	}
	rec, err := a.clientService.CheckLoginPassword(email, form.Password)
	if err != nil {
		clientLoginLimiter.registerFailure(remoteIP, email)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.user.toasts.wrongCredentials"))
		return
	}
	clientLoginLimiter.registerSuccess(remoteIP, email)
	if err := session.SetLoginClient(c, rec.Id); err != nil {
		logger.Warning("user portal: save session:", err)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.user.toasts.wrongCredentials"))
		return
	}
	jsonMsg(c, "", nil)
}

func (a *UserPortalController) logout(c *gin.Context) {
	if err := session.ClearClientSession(c); err != nil {
		logger.Warning("user portal: clear session:", err)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (a *UserPortalController) currentClient(c *gin.Context) (*model.ClientRecord, bool) {
	id, ok := session.GetLoginClientID(c)
	if !ok {
		return nil, false
	}
	rec, err := a.clientService.GetByID(id)
	if err != nil {
		return nil, false
	}
	return rec, true
}

// me returns the self-service view, or a null object when the caller is not
// logged in. It deliberately never answers 401: the end-user portal shares the
// panel's axios client, whose global 401 handler redirects to the admin login —
// a null obj instead lets the portal show its own login form in place.
func (a *UserPortalController) me(c *gin.Context) {
	rec, ok := a.currentClient(c)
	if !ok {
		jsonObj(c, nil, nil)
		return
	}
	jsonObj(c, a.buildView(c, rec), nil)
}

func (a *UserPortalController) buildView(c *gin.Context, rec *model.ClientRecord) ClientSelfView {
	host := resolveHost(c)
	view := ClientSelfView{
		Email:      rec.Email,
		Enable:     rec.Enable,
		Total:      rec.TotalGB,
		ExpiryTime: rec.ExpiryTime,
		SubID:      rec.SubID,
	}
	if traffic, err := a.inboundService.GetClientTrafficByEmail(rec.Email); err == nil && traffic != nil {
		view.Up = traffic.Up
		view.Down = traffic.Down
		view.Total = traffic.Total
		view.ExpiryTime = traffic.ExpiryTime
		view.Enable = traffic.Enable
		view.LastOnline = traffic.LastOnline
	}
	if links, err := a.inboundService.GetAllClientLinks(host, rec.Email); err == nil {
		view.Links = links
	}
	if ext, err := a.clientService.GetExternalLinksForRecord(rec.Id); err == nil {
		view.ExternalLinks = ext
	}
	if ips, err := a.inboundService.GetClientIpsWithNodes(rec.Email); err == nil {
		view.Ips = ips
	}
	if rec.SubID != "" {
		sub, jsonURL, clashURL, _ := a.inboundService.GetSubURLs(host, rec.SubID)
		if enabled, _ := a.settingService.GetSubEnable(); enabled {
			view.SubURL = sub
		}
		if enabled, _ := a.settingService.GetSubJsonEnable(); enabled {
			view.SubJsonURL = jsonURL
		}
		if enabled, _ := a.settingService.GetSubClashEnable(); enabled {
			view.SubClashURL = clashURL
		}
	}
	return view
}

func (a *UserPortalController) rotateSub(c *gin.Context) {
	rec, ok := a.currentClient(c)
	if !ok {
		jsonObj(c, nil, nil)
		return
	}
	newSubID, err := a.clientService.RotateSubID(&a.inboundService, rec.Email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.user.toasts.rotateFailed"), err)
		return
	}
	rec.SubID = newSubID
	jsonObj(c, a.buildView(c, rec), nil)
}
