package domain

import (
	"encoding/json"
	"time"
)

type EgressCellStatus string

const (
	EgressCellActive   EgressCellStatus = "active"
	EgressCellDisabled EgressCellStatus = "disabled"
	EgressCellError    EgressCellStatus = "error"
)

type EgressCell struct {
	ID            string           `db:"id"             json:"id"`
	Name          string           `db:"name"           json:"name"`
	Status        EgressCellStatus `db:"status"         json:"status"`
	ProxyJSON     string           `db:"proxy_json"     json:"-"`
	LabelsJSON    string           `db:"labels_json"    json:"-"`
	StateJSON     string           `db:"state_json"     json:"-"`
	CreatedAt     time.Time        `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time        `db:"updated_at"     json:"updated_at"`
	CooldownUntil *time.Time       `db:"cooldown_until" json:"cooldown_until,omitempty"`

	Proxy  *ProxyConfig      `db:"-" json:"proxy,omitempty"`
	Labels map[string]string `db:"-" json:"labels,omitempty"`
}

func (c *EgressCell) HydrateRuntime() {
	if c == nil {
		return
	}
	if c.ProxyJSON != "" {
		var p ProxyConfig
		if json.Unmarshal([]byte(c.ProxyJSON), &p) == nil && p.Host != "" {
			c.Proxy = &p
		}
	}
	if c.LabelsJSON != "" {
		var labels map[string]string
		if json.Unmarshal([]byte(c.LabelsJSON), &labels) == nil {
			c.Labels = labels
		}
	}
}

func (c *EgressCell) PersistRuntime() {
	if c == nil {
		return
	}
	if c.Proxy != nil {
		data, _ := json.Marshal(c.Proxy)
		c.ProxyJSON = string(data)
	} else {
		c.ProxyJSON = ""
	}
	if c.Labels != nil {
		data, _ := json.Marshal(c.Labels)
		c.LabelsJSON = string(data)
	} else {
		c.LabelsJSON = ""
	}
	if c.StateJSON == "" {
		c.StateJSON = "{}"
	}
	if c.Status == "" {
		c.Status = EgressCellActive
	}
}
