package sub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// writeSubError translates a service-layer result into an HTTP response.
// A nil error with no rows means the subId doesn't match anything (deleted
// client, never-existed id) and becomes 404. A real error becomes 500. No
// body — VPN clients only look at the status.
func writeSubError(c *gin.Context, err error) {
	if err == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusInternalServerError)
}

// SUBController handles HTTP requests for subscription links and JSON configurations.
type SUBController struct {
	subTitle string

	subPath        string
	subJsonPath    string
	subClashPath   string
	jsonEnabled    bool
	clashEnabled   bool
	subEncrypt     bool
	updateInterval string

	subService      *SubService
	subJsonService  *SubJsonService
	subClashService *SubClashService
}

// NewSUBController creates a new subscription controller with the given configuration.
func NewSUBController(
	g *gin.RouterGroup,
	subPath string,
	jsonPath string,
	clashPath string,
	jsonEnabled bool,
	clashEnabled bool,
	encrypt bool,
	remarkTemplate string,
	update string,
	jsonMux string,
	jsonRules string,
	jsonFinalMask string,
	clashEnableRouting bool,
	clashRules string,
	clashTemplate string,
	subTitle string,
) *SUBController {
	sub := NewSubService(remarkTemplate)
	a := &SUBController{
		subTitle: subTitle,

		subPath:        subPath,
		subJsonPath:    jsonPath,
		subClashPath:   clashPath,
		jsonEnabled:    jsonEnabled,
		clashEnabled:   clashEnabled,
		subEncrypt:     encrypt,
		updateInterval: update,

		subService:      sub,
		subJsonService:  NewSubJsonService(jsonMux, jsonRules, jsonFinalMask, sub),
		subClashService: NewSubClashService(clashEnableRouting, clashRules, clashTemplate, sub),
	}
	a.initRouter(g)
	return a
}

// initRouter registers HTTP routes for subscription links and JSON endpoints
// on the provided router group.
func (a *SUBController) initRouter(g *gin.RouterGroup) {
	gLink := g.Group(a.subPath)
	gLink.GET(":subid", a.subs)
	gLink.HEAD(":subid", a.subs)
	if a.jsonEnabled {
		gJson := g.Group(a.subJsonPath)
		gJson.GET(":subid", a.subJsons)
		gJson.HEAD(":subid", a.subJsons)
	}
	if a.clashEnabled {
		gClash := g.Group(a.subClashPath)
		gClash.GET(":subid", a.subClashs)
		gClash.HEAD(":subid", a.subClashs)
	}
}

// subs handles HTTP requests for subscription links, returning either HTML page or base64-encoded subscription data.
func (a *SUBController) subs(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, hostHeader := a.subService.ResolveRequest(c)
	subReq := a.subService.ForRequest(host)
	subReq.SetRequestOrigin(scheme, hostWithPort)
	// The remark template's per-client info is for the content a client app
	// imports — the raw subscription body. A browser viewing the HTML info page
	// gets clean, name-only remarks (usage is shown in the page summary).
	accept := c.GetHeader("Accept")
	wantsHTML := strings.Contains(strings.ToLower(accept), "text/html") || c.Query("html") == "1" || strings.EqualFold(c.Query("view"), "html")
	subReq.subscriptionBody = !wantsHTML
	subs, emails, lastOnline, traffic, err := subReq.getSubs(subId)
	if err != nil || len(subs) == 0 {
		writeSubError(c, err)
	} else {
		var result strings.Builder
		for _, sub := range subs {
			result.WriteString(sub)
			result.WriteString("\n")
		}

		// If the request expects HTML (e.g., browser) or explicitly asked (?html=1 or ?view=html), render the info page here
		if wantsHTML {
			subURL, subJsonURL, subClashURL := subReq.BuildURLs(a.subPath, a.subJsonPath, a.subClashPath, subId)
			if !a.jsonEnabled {
				subJsonURL = ""
			}
			if !a.clashEnabled {
				subClashURL = ""
			}
			basePath, exists := c.Get("base_path")
			if !exists {
				basePath = "/"
			}
			basePathStr := basePath.(string)
			page := subReq.BuildPageData(subId, hostHeader, traffic, lastOnline, subs, emails, subURL, subJsonURL, subClashURL, basePathStr, a.subTitle)
			a.serveSubPage(c, basePathStr, page)
			return
		}

		// Add headers
		header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle)

		if a.subEncrypt {
			c.String(200, base64.StdEncoding.EncodeToString([]byte(result.String())))
		} else {
			c.String(200, result.String())
		}
	}
}

// serveSubPage renders internal/web/dist/subpage.html for the current subscription
// request. The Vite-built SPA reads window.__SUB_PAGE_DATA__ on mount —
// we inject that here, along with window.X_UI_BASE_PATH so the
// page's static asset references resolve correctly when the panel runs
// behind a URL prefix.
func (a *SUBController) serveSubPage(c *gin.Context, basePath string, page PageData) {
	var body []byte
	if diskBody, diskErr := os.ReadFile("internal/web/dist/subpage.html"); diskErr == nil {
		body = diskBody
	} else {
		readBody, err := distFS.ReadFile("dist/subpage.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "missing embedded subpage")
			return
		}
		body = readBody
	}

	// Vite emits absolute asset URLs (`/assets/...`); when the panel is
	// installed under a custom URL prefix, rewrite them so the bundle
	// loads from `<basePath>assets/...` where the static handler is
	// actually mounted.
	if basePath != "/" && basePath != "" {
		body = bytes.ReplaceAll(body, []byte(`src="/assets/`), []byte(`src="`+basePath+`assets/`))
		body = bytes.ReplaceAll(body, []byte(`href="/assets/`), []byte(`href="`+basePath+`assets/`))
	}

	subData := map[string]any{
		"sId":          page.SId,
		"enabled":      page.Enabled,
		"download":     page.Download,
		"upload":       page.Upload,
		"total":        page.Total,
		"used":         page.Used,
		"remained":     page.Remained,
		"expire":       page.Expire,
		"lastOnline":   page.LastOnline,
		"downloadByte": page.DownloadByte,
		"uploadByte":   page.UploadByte,
		"totalByte":    page.TotalByte,
		"subUrl":       page.SubUrl,
		"subJsonUrl":   page.SubJsonUrl,
		"subClashUrl":  page.SubClashUrl,
		"subTitle":     page.SubTitle,
		"links":        page.Result,
		"emails":       page.Emails,
	}

	subDataJSON, err := json.Marshal(subData)
	if err != nil {
		subDataJSON = []byte("{}")
	}

	// Defense-in-depth string-escape for the basePath embed — admin-
	// controlled but cheap to harden.
	jsEscape := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"<", `<`,
		">", `>`,
		"&", `&`,
	)
	escapedBase := jsEscape.Replace(basePath)

	inject := []byte(`<script>window.X_UI_BASE_PATH="` + escapedBase + `";` +
		`window.__SUB_PAGE_DATA__=` + string(subDataJSON) + `;</script></head>`)
	out := bytes.Replace(body, []byte("</head>"), inject, 1)

	setNoCacheHeaders(c)
	c.Data(http.StatusOK, "text/html; charset=utf-8", out)
}

// setNoCacheHeaders marks a subscription page response as non-cacheable so VPN
// clients and browsers always fetch fresh traffic/expiry data.
func setNoCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// subJsons handles HTTP requests for JSON subscription configurations.
func (a *SUBController) subJsons(c *gin.Context) {
	subId := c.Param("subid")
	_, host, _, _ := a.subService.ResolveRequest(c)
	jsonSub, header, err := a.subJsonService.GetJson(subId, host)
	if err != nil || len(jsonSub) == 0 {
		writeSubError(c, err)
	} else {
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle)
		c.String(200, jsonSub)
	}
}

func (a *SUBController) subClashs(c *gin.Context) {
	subId := c.Param("subid")
	_, host, _, _ := a.subService.ResolveRequest(c)
	clashSub, header, err := a.subClashService.GetClash(subId, host)
	if err != nil || len(clashSub) == 0 {
		writeSubError(c, err)
	} else {
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle)
		if a.subTitle != "" {
			// Clash clients commonly use Content-Disposition to choose the imported profile name.
			c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(a.subTitle)))
		}
		c.Data(200, "application/yaml; charset=utf-8", []byte(clashSub))
	}
}

// ApplyCommonHeaders sets common HTTP headers for subscription responses including user info, update interval, and profile title.
func (a *SUBController) ApplyCommonHeaders(c *gin.Context, header, updateInterval, profileTitle string) {
	c.Writer.Header().Set("Subscription-Userinfo", header)
	c.Writer.Header().Set("Profile-Update-Interval", updateInterval)
	if profileTitle != "" {
		c.Writer.Header().Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileTitle)))
	}
}
