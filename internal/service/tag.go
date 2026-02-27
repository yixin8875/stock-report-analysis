package service

import (
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

func GetTags() ([]models.Tag, error) {
	var tags []models.Tag
	err := db.DB.Select(&tags, "SELECT * FROM tags ORDER BY name")
	return tags, err
}

func SaveTag(t models.Tag) error {
	if t.ID == 0 {
		_, err := db.DB.Exec("INSERT INTO tags(name,color) VALUES(?,?)", t.Name, t.Color)
		return err
	}
	_, err := db.DB.Exec("UPDATE tags SET name=?,color=? WHERE id=?", t.Name, t.Color, t.ID)
	return err
}

func DeleteTag(id int64) error {
	_, err := db.DB.Exec("DELETE FROM tags WHERE id=?", id)
	return err
}

func GetArticleTags(articleID int64) ([]models.Tag, error) {
	var tags []models.Tag
	err := db.DB.Select(&tags, "SELECT t.* FROM tags t JOIN article_tags at ON t.id=at.tag_id WHERE at.article_id=?", articleID)
	return tags, err
}

func SetArticleTags(articleID int64, tagIDs []int64) error {
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM article_tags WHERE article_id=?", articleID); err != nil {
		return err
	}
	for _, tid := range tagIDs {
		if _, err := tx.Exec("INSERT INTO article_tags(article_id,tag_id) VALUES(?,?)", articleID, tid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func EnsureTag(name string, color string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	if strings.TrimSpace(color) == "" {
		color = "#6b7280"
	}

	if _, err := db.DB.Exec("INSERT INTO tags(name,color) VALUES(?,?) ON CONFLICT(name) DO NOTHING", name, color); err != nil {
		return 0, err
	}
	var id int64
	if err := db.DB.Get(&id, "SELECT id FROM tags WHERE name=?", name); err != nil {
		return 0, err
	}
	return id, nil
}

func AddTagToArticle(articleID int64, tagID int64) error {
	if articleID <= 0 || tagID <= 0 {
		return nil
	}
	_, err := db.DB.Exec("INSERT INTO article_tags(article_id,tag_id) VALUES(?,?) ON CONFLICT(article_id,tag_id) DO NOTHING", articleID, tagID)
	return err
}
