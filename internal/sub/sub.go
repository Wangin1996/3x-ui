// Package sub provides the subscription surface for the 3x-ui panel. It builds a
// self-contained gin.Engine that serves subscription links / JSON / Clash, and
// mounts it onto the panel's own engine — there is no separate listener or port.
package sub

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// BuildEngine constructs the subscription gin.Engine with its own middleware,
// base_path and static assets. It is served by delegation from the panel engine
// (see RegisterOnPanel), so it keeps a fully independent request context and
// never collides with the panel's own base_path / assets wiring.
func BuildEngine(ss *service.SettingService) (*gin.Engine, error) {
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	engine.Use(gin.Recovery())

	LinksPath, err := ss.GetSubPath()
	if err != nil {
		return nil, err
	}
	JsonPath, err := ss.GetSubJsonPath()
	if err != nil {
		return nil, err
	}
	ClashPath, err := ss.GetSubClashPath()
	if err != nil {
		return nil, err
	}
	subJsonEnable, err := ss.GetSubJsonEnable()
	if err != nil {
		return nil, err
	}
	subClashEnable, err := ss.GetSubClashEnable()
	if err != nil {
		return nil, err
	}

	basePath := LinksPath
	if basePath != "/" && !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	engine.Use(func(c *gin.Context) {
		c.Set("base_path", basePath)
	})

	Encrypt, err := ss.GetSubEncrypt()
	if err != nil {
		return nil, err
	}
	RemarkTemplate, err := ss.GetRemarkTemplate()
	if err != nil {
		RemarkTemplate = ""
	}
	SubUpdates, err := ss.GetSubUpdates()
	if err != nil {
		SubUpdates = "10"
	}
	SubJsonMux, err := ss.GetSubJsonMux()
	if err != nil {
		SubJsonMux = ""
	}
	SubJsonRules, err := ss.GetSubJsonRules()
	if err != nil {
		SubJsonRules = ""
	}
	SubJsonFinalMask, err := ss.GetSubJsonFinalMask()
	if err != nil {
		SubJsonFinalMask = ""
	}
	SubClashEnableRouting, err := ss.GetSubClashEnableRouting()
	if err != nil {
		SubClashEnableRouting = false
	}
	SubClashRules, err := ss.GetSubClashRules()
	if err != nil {
		SubClashRules = ""
	}
	SubClashTemplate, err := ss.GetSubClashTemplate()
	if err != nil {
		SubClashTemplate = ""
	}
	SubTitle, err := ss.GetSubTitle()
	if err != nil {
		SubTitle = ""
	}
	SubSupportUrl, err := ss.GetSubSupportUrl()
	if err != nil {
		SubSupportUrl = ""
	}
	SubProfileUrl, err := ss.GetSubProfileUrl()
	if err != nil {
		SubProfileUrl = ""
	}
	SubAnnounce, err := ss.GetSubAnnounce()
	if err != nil {
		SubAnnounce = ""
	}
	SubEnableRouting, err := ss.GetSubEnableRouting()
	if err != nil {
		return nil, err
	}
	SubRoutingRules, err := ss.GetSubRoutingRules()
	if err != nil {
		SubRoutingRules = ""
	}
	SubHideSettings, err := ss.GetSubHideSettings()
	if err != nil {
		SubHideSettings = false
	}
	SubIncyEnableRouting, err := ss.GetSubIncyEnableRouting()
	if err != nil {
		SubIncyEnableRouting = false
	}
	SubIncyRoutingRules, err := ss.GetSubIncyRoutingRules()
	if err != nil {
		SubIncyRoutingRules = ""
	}

	engine.Use(locale.LocalizerMiddleware())

	var linksPathForAssets string
	if LinksPath == "/" {
		linksPathForAssets = "/assets"
	} else {
		linksPathForAssets = strings.TrimRight(LinksPath, "/") + "/assets"
	}

	var assetsFS http.FileSystem
	if _, err := os.Stat("internal/web/dist/assets"); err == nil {
		assetsFS = http.FS(os.DirFS("internal/web/dist/assets"))
	} else if subFS, err := fs.Sub(distFS, "dist/assets"); err == nil {
		assetsFS = http.FS(subFS)
	} else {
		logger.Error("sub: failed to mount embedded dist assets:", err)
	}

	if assetsFS != nil {
		engine.StaticFS("/assets", assetsFS)
		if linksPathForAssets != "/assets" {
			engine.StaticFS(linksPathForAssets, assetsFS)
		}
		if LinksPath != "/" {
			engine.Use(func(c *gin.Context) {
				path := c.Request.URL.Path
				pathPrefix := strings.TrimRight(LinksPath, "/") + "/"
				if strings.HasPrefix(path, pathPrefix) && strings.Contains(path, "/assets/") {
					_, after, ok := strings.Cut(path, "/assets/")
					if ok && after != "" {
						c.FileFromFS(after, assetsFS)
						c.Abort()
						return
					}
				}
				c.Next()
			})
		}
	}

	g := engine.Group("/")
	NewSUBController(
		g, LinksPath, JsonPath, ClashPath, subJsonEnable, subClashEnable, Encrypt, RemarkTemplate, SubUpdates,
		SubJsonMux, SubJsonRules, SubJsonFinalMask, SubClashEnableRouting, SubClashRules, SubClashTemplate,
		SubTitle, SubSupportUrl, SubProfileUrl, SubAnnounce, SubEnableRouting, SubRoutingRules, SubHideSettings,
		SubIncyEnableRouting, SubIncyRoutingRules,
	)

	return engine, nil
}

// RegisterOnPanel mounts the subscription engine onto the panel's engine by
// delegating the configured subscription path prefixes to it. The subscription
// therefore rides the panel's own port and TLS — no separate listener. Paths are
// at the site root (not under the panel basePath), so a shared link never leaks
// the admin panel path.
func RegisterOnPanel(panel *gin.Engine, ss *service.SettingService) error {
	subEnable, err := ss.GetSubEnable()
	if err != nil {
		return err
	}
	if !subEnable {
		return nil
	}
	engine, err := BuildEngine(ss)
	if err != nil {
		return err
	}
	h := gin.WrapH(engine)
	register := func(p string) {
		p = strings.TrimRight(p, "/")
		if p == "" {
			return
		}
		panel.Any(p+"/*any", h)
	}

	linksPath, _ := ss.GetSubPath()
	register(linksPath)
	if enabled, _ := ss.GetSubJsonEnable(); enabled {
		jsonPath, _ := ss.GetSubJsonPath()
		register(jsonPath)
	}
	if enabled, _ := ss.GetSubClashEnable(); enabled {
		clashPath, _ := ss.GetSubClashPath()
		register(clashPath)
	}
	return nil
}
