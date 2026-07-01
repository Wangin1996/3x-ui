package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// nodeBulkPushThreshold caps how many per-client operations a single call streams
// to a node before it marks the node dirty and lets one config pull converge the
// whole inbound instead. Small ops stay on the live per-client path.
const nodeBulkPushThreshold = 32

func (s *InboundService) runtimeFor(ib *model.Inbound) (runtime.Runtime, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager not initialised")
	}
	return mgr.RuntimeFor(ib.NodeID)
}

func (s *InboundService) nodePushPlan(ib *model.Inbound) (runtime.Runtime, bool, bool, error) {
	if ib.NodeID == nil {
		rt, err := s.runtimeFor(ib)
		if err != nil {
			return nil, false, false, nil
		}
		return rt, true, false, nil
	}
	nodeSvc := NodeService{}
	enabled, status, _, _, err := nodeSvc.NodeSyncState(*ib.NodeID)
	if err != nil {
		return nil, false, false, err
	}
	if !enabled || status == "offline" {
		return nil, false, true, nil
	}
	rt, err := s.runtimeFor(ib)
	if err != nil {
		return nil, false, true, nil
	}
	return rt, true, false, nil
}

func (s *InboundService) NodeIsPending(nodeID *int) bool {
	if nodeID == nil {
		return false
	}
	return (&NodeService{}).IsNodePending(*nodeID)
}

func (s *InboundService) AnyNodePending(inboundIds []int) bool {
	if len(inboundIds) == 0 {
		return false
	}
	nodeSvc := NodeService{}
	for _, id := range inboundIds {
		ib, err := s.GetInbound(id)
		if err != nil || ib.NodeID == nil {
			continue
		}
		if nodeSvc.IsNodePending(*ib.NodeID) {
			return true
		}
	}
	return false
}

// onlineGracePeriodMs must comfortably exceed the 5s traffic-poll interval —
// Xray's stats counters often report a zero delta for an active session across
// a single poll, so a 5s grace would still drop the client on the next tick.
// ~4 polls of slack keeps idle-but-connected clients visible without lingering
// long after a real disconnect.
const onlineGracePeriodMs int64 = 20000

type nodeTrafficCounter struct {
	Up   int64
	Down int64
}

func (s *InboundService) upsertNodeBaseline(tx *gorm.DB, nodeID int, email string, up, down int64) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}, {Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"up", "down"}),
	}).Create(&model.NodeClientTraffic{NodeId: nodeID, Email: email, Up: up, Down: down}).Error
}

// GetNodeInboundTrafficTotals returns the current cumulative up/down for every
// node-hosted inbound, keyed by tag. The node sync diffs successive snapshots of
// this to derive per-inbound speed for the dashboard — node inbounds have no
// local Xray poll to produce live deltas the way local inbounds do.
func (s *InboundService) GetNodeInboundTrafficTotals() (map[string][2]int64, error) {
	var rows []struct {
		Tag  string
		Up   int64
		Down int64
	}
	if err := database.GetDB().Table("inbounds").
		Select("tag, up, down").
		Where("node_id IS NOT NULL").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string][2]int64, len(rows))
	for _, r := range rows {
		out[r.Tag] = [2]int64{r.Up, r.Down}
	}
	return out, nil
}

func (s *InboundService) GetOnlineClients() []string {
	if p == nil {
		return []string{}
	}
	return p.GetOnlineClients()
}

// GetOnlineClientsByGuid returns online emails keyed by the panelGuid of the
// node that physically hosts each set: this panel's own clients under its own
// GUID, plus every node in the tree under its GUID (#4983). Replaces the old
// node-id keying so a client three hops down is attributed to its real node,
// not the intermediate one it was synced through.
func (s *InboundService) GetOnlineClientsByGuid() map[string][]string {
	if p == nil {
		return map[string][]string{}
	}
	out := p.GetMergedNodeTrees()
	if local := p.GetLocalOnlineClients(); len(local) > 0 {
		if guid := s.panelGuid(); guid != "" {
			out[guid] = mergeEmails(out[guid], local)
		}
	}
	return out
}

// GetActiveInboundsByGuid returns the inbound tags that carried traffic within
// the grace window for THIS panel, under its own GUID. Remote nodes don't
// report per-inbound activity, so a GUID missing from the map means "don't
// gate" for that node's inbounds.
func (s *InboundService) GetActiveInboundsByGuid() map[string][]string {
	if p == nil {
		return map[string][]string{}
	}
	active := p.GetLocalActiveInbounds()
	if len(active) == 0 {
		return map[string][]string{}
	}
	guid := s.panelGuid()
	if guid == "" {
		return map[string][]string{}
	}
	return map[string][]string{guid: active}
}

func (s *InboundService) SetNodeOnlineTree(nodeID int, tree map[string][]string) {
	if p != nil {
		p.SetNodeOnlineTree(nodeID, tree)
	}
}

func (s *InboundService) ClearNodeOnlineClients(nodeID int) {
	if p != nil {
		p.ClearNodeOnlineClients(nodeID)
	}
}

// panelGuid returns this panel's stable self-identifier, used to key the local
// panel's own clients in the per-node online maps (#4983).
func (s *InboundService) panelGuid() string {
	guid, _ := (&SettingService{}).GetPanelGuid()
	return guid
}

// synthNodeGuid is the stable per-node fallback identity for a directly-attached
// node whose panel hasn't reported a panelGuid yet (old build). Node ids are
// master-local, so this only composes for direct nodes — exactly the pre-#4983
// flat-topology case where an old-build node appears.
func synthNodeGuid(nodeID int) string {
	return fmt.Sprintf("node:%d", nodeID)
}

// mergeEmails returns the deduped union of two email slices.
func mergeEmails(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, e := range a {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	for _, e := range b {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}

func (s *InboundService) GetClientsLastOnline() (map[string]int64, error) {
	db := database.GetDB()
	var rows []xray.ClientTraffic
	err := db.Model(&xray.ClientTraffic{}).Select("email, last_online").Find(&rows).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Email] = r.LastOnline
	}
	return result, nil
}

// RefreshLocalOnlineClients folds the emails and inbound tags active on this
// panel's own xray this poll into the local online/active sets, applying the
// grace window and pruning stale entries. Pass nil to only prune. See
// xray.Process for why the local sets are kept separate from the shared
// last_online column.
func (s *InboundService) RefreshLocalOnlineClients(activeEmails, activeInboundTags []string) {
	if p != nil {
		p.RefreshLocalOnline(activeEmails, activeInboundTags, time.Now().UnixMilli(), onlineGracePeriodMs)
	}
}

func (s *InboundService) FilterAndSortClientEmails(emails []string) ([]string, []string, error) {
	db := database.GetDB()

	// Step 1: Get ClientTraffic records for emails in the input list.
	// Chunked to stay under SQLite's bind-variable limit on huge inputs.
	uniqEmails := uniqueNonEmptyStrings(emails)
	clients := make([]xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []xray.ClientTraffic
		if err := db.Where("email IN ?", batch).Find(&page).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		clients = append(clients, page...)
	}

	// Step 2: Sort clients by (Up + Down) descending
	sort.Slice(clients, func(i, j int) bool {
		return (clients[i].Up + clients[i].Down) > (clients[j].Up + clients[j].Down)
	})

	// Step 3: Extract sorted valid emails and track found ones
	validEmails := make([]string, 0, len(clients))
	found := make(map[string]bool)
	for _, client := range clients {
		validEmails = append(validEmails, client.Email)
		found[client.Email] = true
	}

	// Step 4: Identify emails that were not found in the database
	extraEmails := make([]string, 0)
	for _, email := range emails {
		if !found[email] {
			extraEmails = append(extraEmails, email)
		}
	}

	return validEmails, extraEmails, nil
}
