package service

import (
	"errors"
	"fmt"
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

var builtinRoleTemplates = []models.RoleTemplate{
	{
		ID:           "financial-analyst",
		Name:         "财务分析师",
		Alias:        "finance",
		DomainTags:   "财务,现金流,利润质量,资产负债表",
		SystemPrompt: "你是财务分析师。优先从利润表、资产负债表、现金流量表进行拆解，明确结论、证据、风险。避免空泛结论。",
	},
	{
		ID:           "industry-analyst",
		Name:         "行业分析师",
		Alias:        "industry",
		DomainTags:   "行业景气,竞争格局,供需,份额",
		SystemPrompt: "你是行业分析师。重点解释行业景气周期、竞争格局、供需变化和公司在产业链位置，给出趋势判断与关键变量。",
	},
	{
		ID:           "risk-officer",
		Name:         "风险官",
		Alias:        "risk",
		DomainTags:   "风险,回撤,不确定性,假设脆弱点",
		SystemPrompt: "你是风险官。优先识别假设脆弱点、负面情景和触发条件，给出可监控指标与预警信号。",
	},
	{
		ID:           "trading-strategist",
		Name:         "交易策略师",
		Alias:        "trader",
		DomainTags:   "交易节奏,催化,仓位,事件驱动",
		SystemPrompt: "你是交易策略师。强调时间窗口、催化事件、预期差和仓位管理，输出可执行但不过度确定性的策略建议。",
	},
	{
		ID:           "valuation-specialist",
		Name:         "估值分析师",
		Alias:        "valuation",
		DomainTags:   "估值,可比公司,折现,安全边际",
		SystemPrompt: "你是估值分析师。聚焦估值框架、关键参数敏感性和安全边际，解释估值结论与区间变化原因。",
	},
}

func GetRoleTemplates() []models.RoleTemplate {
	out := make([]models.RoleTemplate, len(builtinRoleTemplates))
	copy(out, builtinRoleTemplates)
	return out
}

func CreateRoleFromTemplate(templateID string) (models.Role, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return models.Role{}, errors.New("模板 ID 不能为空")
	}

	var tpl *models.RoleTemplate
	for i := range builtinRoleTemplates {
		if builtinRoleTemplates[i].ID == templateID {
			tpl = &builtinRoleTemplates[i]
			break
		}
	}
	if tpl == nil {
		return models.Role{}, errors.New("未找到角色模板")
	}

	name, err := nextAvailableRoleName(tpl.Name)
	if err != nil {
		return models.Role{}, err
	}
	alias := tpl.Alias
	if name != tpl.Name {
		alias = ""
	}

	role := models.Role{
		Name:          name,
		Alias:         alias,
		DomainTags:    tpl.DomainTags,
		SystemPrompt:  tpl.SystemPrompt,
		ModelOverride: "",
		Temperature:   0.2,
		MaxTokens:     1200,
		Enabled:       1,
		IsDefault:     0,
	}
	if err := SaveRole(role); err != nil {
		return models.Role{}, err
	}

	var created models.Role
	if err := db.DB.Get(&created, `SELECT * FROM roles WHERE name=? ORDER BY id DESC LIMIT 1`, name); err != nil {
		return models.Role{}, err
	}
	return created, nil
}

func nextAvailableRoleName(base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", errors.New("模板名称无效")
	}
	name := base
	for i := 0; i < 100; i++ {
		var cnt int
		if err := db.DB.Get(&cnt, `SELECT COUNT(*) FROM roles WHERE name=?`, name); err != nil {
			return "", err
		}
		if cnt == 0 {
			return name, nil
		}
		name = fmt.Sprintf("%s %d", base, i+2)
	}
	return "", errors.New("角色名称冲突过多，请手动创建")
}
