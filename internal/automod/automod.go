package automod

import "github.com/discuitnet/discuit/internal/uid"

type AutoModFlag struct {
	ID           int    `json:"id"`
	ContentTable string `json:"content_table"`
	ContentId    uid.ID `json:"content_id"`
	Reason       int    `json:"reason"`       // Number corallated to a reason, such as 1 for spam, 2 for offensive, 3 for being caught by a custom rule, etc.
	ActionTaken  int    `json:"action_taken"` // Bases on the rule's action, such as 1 for alert, 2 for hide, 3 for delete, etc.
	Cleared      bool   `json:"cleared"`      // If the flag has been cleared by a mod
	Notes        string `json:"notes"`        // Notes added by a mod
}

type AutoModConfig struct {
	CommunityID string `json:"community_id"`
	Active      bool   `json:"active"`
	ConfigJson  string `json:"config_json"`
}

type AutoModConfigJson struct {
	CustomRules []AutoModCustomRule `json:"custom_rules"`
}

type AutoModCustomRule struct {
	Type   string `json:"type"` // "keyword", "phrase", "regex"
	Value  string `json:"value"`
	Action string `json:"action"` // "none", "alert", "hide", "delete"
}
