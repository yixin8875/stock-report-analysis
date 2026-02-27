package service

import (
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"

	"github.com/jmoiron/sqlx"
)

var mentionPattern = regexp.MustCompile(`@([^\s@,，。:：;；!！?？]+)`)

const fallbackRoleName = "通用分析师"
const fallbackRolePrompt = "你是资深股票研究分析师。请基于用户提供的报告上下文回答问题。禁止编造事实；若证据不足要明确说明。回答要结构清晰，先结论后理由。"

func GetRoles() ([]models.Role, error) {
	var roles []models.Role
	err := db.DB.Select(&roles, `
		SELECT *
		FROM roles
		ORDER BY is_default DESC, enabled DESC, id ASC
	`)
	return roles, err
}

func GetDefaultRole() (models.Role, error) {
	var role models.Role
	err := db.DB.Get(&role, `
		SELECT *
		FROM roles
		WHERE enabled = 1 AND is_default = 1
		ORDER BY id ASC
		LIMIT 1
	`)
	return role, err
}

func SaveRole(role models.Role) error {
	role.Name = strings.TrimSpace(role.Name)
	role.Alias = strings.TrimSpace(role.Alias)
	role.DomainTags = strings.TrimSpace(role.DomainTags)
	role.SystemPrompt = strings.TrimSpace(role.SystemPrompt)
	role.ModelOverride = strings.TrimSpace(role.ModelOverride)

	if role.Name == "" {
		return errors.New("角色名称不能为空")
	}
	if role.SystemPrompt == "" {
		return errors.New("角色系统提示词不能为空")
	}
	if role.Enabled == 0 && role.IsDefault == 1 {
		return errors.New("默认角色必须启用")
	}
	if role.MaxTokens <= 0 {
		role.MaxTokens = 1200
	}
	if role.Temperature <= 0 {
		role.Temperature = 0.2
	}

	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if role.IsDefault == 1 {
		if _, err := tx.Exec("UPDATE roles SET is_default = 0, updated_at = CURRENT_TIMESTAMP WHERE is_default = 1"); err != nil {
			return err
		}
	}

	if role.ID == 0 {
		_, err := tx.Exec(`
			INSERT INTO roles(name, alias, domain_tags, system_prompt, model_override, temperature, max_tokens, enabled, is_default)
			VALUES(?,?,?,?,?,?,?,?,?)
		`, role.Name, role.Alias, role.DomainTags, role.SystemPrompt, role.ModelOverride, role.Temperature, role.MaxTokens, role.Enabled, role.IsDefault)
		if err != nil {
			return err
		}
	} else {
		_, err := tx.Exec(`
			UPDATE roles
			SET name=?, alias=?, domain_tags=?, system_prompt=?, model_override=?, temperature=?, max_tokens=?, enabled=?, is_default=?, updated_at=CURRENT_TIMESTAMP
			WHERE id=?
		`, role.Name, role.Alias, role.DomainTags, role.SystemPrompt, role.ModelOverride, role.Temperature, role.MaxTokens, role.Enabled, role.IsDefault, role.ID)
		if err != nil {
			return err
		}
	}

	if err := ensureDefaultRoleTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func DeleteRole(id int64) error {
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM roles WHERE id=?", id); err != nil {
		return err
	}
	if err := ensureDefaultRoleTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func SetDefaultRole(id int64) error {
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var cnt int
	if err := tx.Get(&cnt, "SELECT COUNT(*) FROM roles WHERE id=?", id); err != nil {
		return err
	}
	if cnt == 0 {
		return errors.New("角色不存在")
	}

	if _, err := tx.Exec("UPDATE roles SET is_default=0, updated_at=CURRENT_TIMESTAMP WHERE is_default=1"); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE roles SET enabled=1, is_default=1, updated_at=CURRENT_TIMESTAMP WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func ResolveRolesByMentions(question string) ([]models.Role, string, error) {
	roles, err := GetRoles()
	if err != nil {
		return nil, "", err
	}

	enabled := make([]models.Role, 0, len(roles))
	for _, role := range roles {
		if role.Enabled == 1 {
			enabled = append(enabled, role)
		}
	}
	if len(enabled) == 0 {
		return nil, "", errors.New("没有可用角色，请先在设置中启用角色")
	}

	matches := mentionPattern.FindAllStringSubmatch(question, -1)
	roleByKey := make(map[string]models.Role, len(enabled)*2)
	for _, role := range enabled {
		roleByKey[strings.ToLower(role.Name)] = role
		if role.Alias != "" {
			roleByKey[strings.ToLower(role.Alias)] = role
		}
	}

	selected := make([]models.Role, 0)
	seen := make(map[int64]struct{})
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		if key == "" {
			continue
		}
		role, ok := roleByKey[key]
		if !ok {
			continue
		}
		if _, ok := seen[role.ID]; ok {
			continue
		}
		selected = append(selected, role)
		seen[role.ID] = struct{}{}
	}

	cleaned := strings.TrimSpace(mentionPattern.ReplaceAllString(question, ""))
	cleaned = normalizeSpaces(cleaned)
	if cleaned == "" {
		cleaned = strings.TrimSpace(question)
	}

	if len(selected) == 0 {
		def, err := GetDefaultRole()
		if err == nil {
			return []models.Role{def}, cleaned, nil
		}
		// fallback to first enabled role
		sort.Slice(enabled, func(i, j int) bool {
			return enabled[i].ID < enabled[j].ID
		})
		return []models.Role{enabled[0]}, cleaned, nil
	}
	return selected, cleaned, nil
}

func ensureDefaultRoleTx(tx *sqlx.Tx) error {
	var cnt int
	if err := tx.Get(&cnt, "SELECT COUNT(*) FROM roles WHERE enabled=1 AND is_default=1"); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}

	var id int64
	err := tx.Get(&id, "SELECT id FROM roles WHERE enabled=1 ORDER BY id ASC LIMIT 1")
	if err == nil {
		_, err = tx.Exec("UPDATE roles SET is_default=1, updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO roles(name, alias, domain_tags, system_prompt, model_override, temperature, max_tokens, enabled, is_default)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, fallbackRoleName, "general", "通用,基本面", fallbackRolePrompt, "", 0.2, 1200, 1, 1)
	return err
}

func normalizeSpaces(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
