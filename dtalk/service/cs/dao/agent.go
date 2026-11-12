package dao

import (
	"github.com/web3chat/txchat/dtalk/service/cs/model"
	"time"
)

const (
	_getAgentList      = `SELECT id, agent_id, username, name, avatar, status, sort_order, created_at, updated_at FROM cs_agent ORDER BY sort_order ASC, id ASC`
	_getAgentByID      = `SELECT id, agent_id, username, password_hash, private_key_enc, public_key, name, avatar, status, sort_order, created_at, updated_at FROM cs_agent WHERE agent_id = ?`
	_getAgentByUser    = `SELECT id, agent_id, username, password_hash, private_key_enc, public_key, name, avatar, status, sort_order, created_at, updated_at FROM cs_agent WHERE username = ?`
	_updateAgentStatus = `UPDATE cs_agent SET status = ? WHERE agent_id = ?`
)

func scanFullAgent(r map[string]string) *model.CSAgent {
	a := &model.CSAgent{
		AgentID:       r["agent_id"],
		Username:      r["username"],
		PasswordHash:  r["password_hash"],
		PrivateKeyEnc: r["private_key_enc"],
		PublicKey:     r["public_key"],
		Name:          r["name"],
		Avatar:        r["avatar"],
	}
	if v, err := parseInt64(r["id"]); err == nil {
		a.ID = v
	}
	if v, err := parseInt8(r["status"]); err == nil {
		a.Status = v
	}
	if v, err := parseInt(r["sort_order"]); err == nil {
		a.SortOrder = v
	}
	if v, err := time.Parse("2006-01-02 15:04:05", r["created_at"]); err == nil {
		a.CreatedAt = v
	}
	if v, err := time.Parse("2006-01-02 15:04:05", r["updated_at"]); err == nil {
		a.UpdatedAt = v
	}
	return a
}

func (d *Dao) GetAgentList() ([]*model.CSAgent, error) {
	records, err := d.conn.Query(_getAgentList)
	if err != nil {
		return nil, err
	}

	var list []*model.CSAgent
	for _, r := range records {
		a := &model.CSAgent{
			AgentID: r["agent_id"],
			Name:    r["name"],
			Avatar:  r["avatar"],
		}
		if v, err := parseInt64(r["id"]); err == nil {
			a.ID = v
		}
		if v, err := parseInt8(r["status"]); err == nil {
			a.Status = v
		}
		if v, err := parseInt(r["sort_order"]); err == nil {
			a.SortOrder = v
		}
		list = append(list, a)
	}
	return list, nil
}

func (d *Dao) GetAgentByID(agentID string) (*model.CSAgent, error) {
	records, err := d.conn.Query(_getAgentByID, agentID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return scanFullAgent(records[0]), nil
}

func (d *Dao) GetAgentByUsername(username string) (*model.CSAgent, error) {
	records, err := d.conn.Query(_getAgentByUser, username)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return scanFullAgent(records[0]), nil
}

func (d *Dao) UpdateAgentStatus(agentID string, status int8) error {
	_, _, err := d.conn.Exec(_updateAgentStatus, status, agentID)
	return err
}
