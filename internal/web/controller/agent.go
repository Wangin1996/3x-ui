package controller

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/agent/wire"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"

	"github.com/gin-gonic/gin"
)

const agentNodeContextKey = "agent_node"

// AgentController serves the pull-mode node agent surface: a node agent dials in,
// pulls its rendered Xray config, and reports traffic/online/system state back.
type AgentController struct {
	BaseController

	nodeService    service.NodeService
	xrayService    service.XrayService
	inboundService service.InboundService
}

// NewAgentController creates the agent controller and registers its routes.
func NewAgentController(g *gin.RouterGroup) *AgentController {
	a := &AgentController{}
	a.initRouter(g)
	return a
}

func (a *AgentController) initRouter(g *gin.RouterGroup) {
	g.Use(a.agentAuth)
	g.GET("/config", a.config)
	g.POST("/report", a.report)
	g.GET("/ws", a.ws)
}

func (a *AgentController) ws(c *gin.Context) {
	node := c.MustGet(agentNodeContextKey).(*model.Node)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("agent ws: upgrade failed:", err)
		return
	}
	ac := websocket.RegisterAgent(node.Id, conn)
	_ = a.nodeService.UpdateHeartbeat(node.Id, service.HeartbeatPatch{Status: "online", LastHeartbeat: time.Now().Unix()})
	defer func() {
		websocket.UnregisterAgent(node.Id, ac)
		_ = conn.Close()
		_ = a.nodeService.UpdateHeartbeat(node.Id, service.HeartbeatPatch{Status: "offline", LastHeartbeat: time.Now().Unix()})
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (a *AgentController) agentAuth(c *gin.Context) {
	guid := c.GetHeader(wire.HeaderNodeGuid)
	token := agentBearerToken(c)
	if guid == "" || token == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	node, err := a.nodeService.GetByGuid(guid)
	if err != nil || node == nil || !node.Enable || node.Mode != "agent" || node.ApiToken == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(node.ApiToken)) != 1 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Set("api_authed", true)
	c.Set(agentNodeContextKey, node)
	c.Next()
}

func agentBearerToken(c *gin.Context) string {
	if after, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
		return after
	}
	return ""
}

func (a *AgentController) config(c *gin.Context) {
	node := c.MustGet(agentNodeContextKey).(*model.Node)
	cfg, err := a.xrayService.RenderAgentConfig(node.Id)
	if err != nil {
		jsonMsg(c, "render agent config", err)
		return
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		jsonMsg(c, "marshal agent config", err)
		return
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	if node.ConfigDirty {
		if err := a.nodeService.ClearNodeDirty(node.Id, node.ConfigDirtyAt); err != nil {
			logger.Warning("agent config: clear dirty failed:", err)
		}
	}
	if c.Query("sha") == sha {
		jsonObj(c, wire.ConfigResponse{Unchanged: true, Sha256: sha}, nil)
		return
	}
	jsonObj(c, wire.ConfigResponse{Sha256: sha, Config: cfg}, nil)
}

func (a *AgentController) report(c *gin.Context) {
	node := c.MustGet(agentNodeContextKey).(*model.Node)
	var report wire.Report
	if err := c.ShouldBindJSON(&report); err != nil {
		jsonMsg(c, "decode agent report", err)
		return
	}

	if err := a.inboundService.IngestAgentTraffic(node.Id, report.ClientTraffics); err != nil {
		jsonMsg(c, "ingest agent traffic", err)
		return
	}
	a.inboundService.SetNodeOnlineEmails(node, report.OnlineEmails)

	if err := a.nodeService.UpdateHeartbeat(node.Id, service.HeartbeatPatch{
		Status:        "online",
		LastHeartbeat: time.Now().Unix(),
		XrayVersion:   report.Sys.XrayVersion,
		CpuPct:        report.Sys.CpuPct,
		MemPct:        report.Sys.MemPct,
		UptimeSecs:    report.Sys.UptimeSecs,
		NetUp:         report.Sys.NetUp,
		NetDown:       report.Sys.NetDown,
		XrayState:     report.Sys.XrayState,
		XrayError:     report.Sys.XrayError,
	}); err != nil {
		logger.Warning("agent report: update heartbeat failed:", err)
	}

	globals, err := a.inboundService.GetNodeClientTraffics(node.Id)
	if err != nil {
		logger.Warning("agent report: load globals failed:", err)
	}
	jsonObj(c, wire.ReportResponse{Ok: true, Globals: globals, ConfigDirty: node.ConfigDirty}, nil)
}
