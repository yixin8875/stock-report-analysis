package service

import (
	"errors"
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"

	"github.com/jmoiron/sqlx"
)

func GetChannels() ([]models.AIChannel, error) {
	var channels []models.AIChannel
	err := db.DB.Select(&channels, "SELECT * FROM ai_channels ORDER BY id DESC")
	return channels, err
}

func SaveChannel(ch models.AIChannel) error {
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if ch.IsDefault == 1 {
		if _, err := tx.Exec("UPDATE ai_channels SET is_default=0 WHERE is_default=1"); err != nil {
			return err
		}
	}
	if ch.ID == 0 {
		if _, err := tx.Exec("INSERT INTO ai_channels(name,base_url,api_key,model,is_default) VALUES(?,?,?,?,?)",
			ch.Name, ch.BaseURL, ch.APIKey, ch.Model, ch.IsDefault); err != nil {
			return err
		}
		return tx.Commit()
	}
	if _, err := tx.Exec("UPDATE ai_channels SET name=?,base_url=?,api_key=?,model=?,is_default=? WHERE id=?",
		ch.Name, ch.BaseURL, ch.APIKey, ch.Model, ch.IsDefault, ch.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func DeleteChannel(id int64) error {
	_, err := db.DB.Exec("DELETE FROM ai_channels WHERE id=?", id)
	return err
}

func GetPrompts() ([]models.Prompt, error) {
	var prompts []models.Prompt
	err := db.DB.Select(&prompts, "SELECT * FROM prompts ORDER BY id DESC")
	return prompts, err
}

func SavePrompt(p models.Prompt) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("提示词名称不能为空")
	}
	if strings.TrimSpace(p.Content) == "" {
		return errors.New("提示词内容不能为空")
	}

	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if p.IsDefault == 1 {
		if _, err := tx.Exec("UPDATE prompts SET is_default=0 WHERE is_default=1"); err != nil {
			return err
		}
	}
	if p.ID == 0 {
		res, err := tx.Exec("INSERT INTO prompts(name,content,is_default) VALUES(?,?,?)",
			p.Name, p.Content, p.IsDefault)
		if err != nil {
			return err
		}
		promptID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertPromptVersionTx(tx, promptID, p.Name, p.Content); err != nil {
			return err
		}
		return tx.Commit()
	}

	var before models.Prompt
	if err := tx.Get(&before, "SELECT * FROM prompts WHERE id=?", p.ID); err != nil {
		return err
	}

	if _, err := tx.Exec("UPDATE prompts SET name=?,content=?,is_default=? WHERE id=?",
		p.Name, p.Content, p.IsDefault, p.ID); err != nil {
		return err
	}
	if before.Name != p.Name || before.Content != p.Content {
		if err := insertPromptVersionTx(tx, p.ID, p.Name, p.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func DeletePrompt(id int64) error {
	_, err := db.DB.Exec("DELETE FROM prompts WHERE id=?", id)
	return err
}

func GetPromptVersions(promptID int64) ([]models.PromptVersion, error) {
	if promptID <= 0 {
		return nil, errors.New("提示词 ID 无效")
	}
	var versions []models.PromptVersion
	err := db.DB.Select(&versions, `
		SELECT *
		FROM prompt_versions
		WHERE prompt_id=?
		ORDER BY version_no DESC, id DESC
	`, promptID)
	return versions, err
}

func RestorePromptVersion(promptID int64, versionID int64) error {
	if promptID <= 0 || versionID <= 0 {
		return errors.New("提示词版本参数无效")
	}

	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var version models.PromptVersion
	if err := tx.Get(&version, "SELECT * FROM prompt_versions WHERE id=? AND prompt_id=?", versionID, promptID); err != nil {
		return err
	}

	var before models.Prompt
	if err := tx.Get(&before, "SELECT * FROM prompts WHERE id=?", promptID); err != nil {
		return err
	}

	if _, err := tx.Exec("UPDATE prompts SET name=?,content=? WHERE id=?", version.Name, version.Content, promptID); err != nil {
		return err
	}

	// Record restore as a new version so rollback history itself is traceable.
	if before.Name != version.Name || before.Content != version.Content {
		if err := insertPromptVersionTx(tx, promptID, version.Name, version.Content); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func insertPromptVersionTx(tx *sqlx.Tx, promptID int64, name string, content string) error {
	var nextVersion int
	if err := tx.Get(&nextVersion, "SELECT COALESCE(MAX(version_no), 0) + 1 FROM prompt_versions WHERE prompt_id=?", promptID); err != nil {
		return err
	}
	_, err := tx.Exec(`
		INSERT INTO prompt_versions(prompt_id, version_no, name, content)
		VALUES(?,?,?,?)
	`, promptID, nextVersion, name, content)
	return err
}
