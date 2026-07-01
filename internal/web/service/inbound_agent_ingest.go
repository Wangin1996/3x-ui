package service

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// IngestAgentTraffic accumulates per-client traffic reported by a pull-mode node
// agent. The agent reports cumulative-since-start counters per email; this method
// computes the delta against the stored (node, email) baseline and adds it to the
// shared client_traffics row, then advances the baseline. Unlike SetRemoteTraffic
// it NEVER creates, updates, or deletes inbounds or client rows: for agent nodes
// the master is config-authoritative, so a report only moves traffic counters for
// clients the master already owns (UPDATE ... WHERE email is a no-op otherwise).
func (s *InboundService) IngestAgentTraffic(nodeID int, clientTraffics []*xray.ClientTraffic) error {
	if len(clientTraffics) == 0 {
		return nil
	}
	return submitTrafficWrite(func() error {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			var baselineRows []model.NodeClientTraffic
			if err := tx.Where("node_id = ?", nodeID).Find(&baselineRows).Error; err != nil {
				return err
			}
			baselines := make(map[string]nodeTrafficCounter, len(baselineRows))
			for _, b := range baselineRows {
				baselines[b.Email] = nodeTrafficCounter{Up: b.Up, Down: b.Down}
			}

			lastOnlineExpr := database.GreatestExpr("last_online", "?")
			for _, ct := range clientTraffics {
				if ct == nil || ct.Email == "" {
					continue
				}
				base, seen := baselines[ct.Email]
				var deltaUp, deltaDown int64
				if seen {
					if deltaUp = ct.Up - base.Up; deltaUp < 0 {
						deltaUp = 0
					}
					if deltaDown = ct.Down - base.Down; deltaDown < 0 {
						deltaDown = 0
					}
				}
				if deltaUp != 0 || deltaDown != 0 || ct.LastOnline > 0 {
					if err := tx.Exec(
						fmt.Sprintf(`UPDATE client_traffics SET up = up + ?, down = down + ?, last_online = %s WHERE email = ?`, lastOnlineExpr),
						deltaUp, deltaDown, ct.LastOnline, ct.Email,
					).Error; err != nil {
						return err
					}
				}
				if err := s.upsertNodeBaseline(tx, nodeID, ct.Email, ct.Up, ct.Down); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// SetNodeOnlineEmails records the online client set an agent reported, keyed by
// the node's stable guid (admin-provisioned, or a synthetic per-node fallback)
// so the master's online view attributes them to the physical agent node.
func (s *InboundService) SetNodeOnlineEmails(node *model.Node, emails []string) {
	if node == nil {
		return
	}
	guid := node.Guid
	if guid == "" {
		guid = synthNodeGuid(node.Id)
	}
	s.SetNodeOnlineTree(node.Id, map[string][]string{guid: emails})
}
