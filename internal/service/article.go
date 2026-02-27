package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"

	"github.com/jmoiron/sqlx"
)

const telegraphSourcePrefixLike = "cls-telegraph:%"

func GetArticles(keyword string, tagID int64) ([]models.Article, error) {
	var articles []models.Article
	var err error

	if tagID > 0 && keyword != "" {
		q := "%" + keyword + "%"
		err = db.DB.Select(&articles, "SELECT a.id,a.title,a.source,a.status,a.created_at,a.analyzed_at FROM articles a JOIN article_tags at ON a.id=at.article_id WHERE at.tag_id=? AND a.source NOT LIKE ? AND (a.title LIKE ? OR a.content LIKE ?) ORDER BY a.id DESC", tagID, telegraphSourcePrefixLike, q, q)
	} else if tagID > 0 {
		err = db.DB.Select(&articles, "SELECT a.id,a.title,a.source,a.status,a.created_at,a.analyzed_at FROM articles a JOIN article_tags at ON a.id=at.article_id WHERE at.tag_id=? AND a.source NOT LIKE ? ORDER BY a.id DESC", tagID, telegraphSourcePrefixLike)
	} else if keyword != "" {
		q := "%" + keyword + "%"
		err = db.DB.Select(&articles, "SELECT id,title,source,status,created_at,analyzed_at FROM articles WHERE source NOT LIKE ? AND (title LIKE ? OR content LIKE ?) ORDER BY id DESC", telegraphSourcePrefixLike, q, q)
	} else {
		err = db.DB.Select(&articles, "SELECT id,title,source,status,created_at,analyzed_at FROM articles WHERE source NOT LIKE ? ORDER BY id DESC", telegraphSourcePrefixLike)
	}
	if err != nil {
		return nil, err
	}
	if err := loadArticleTags(&articles); err != nil {
		return nil, err
	}
	return articles, nil
}

func GetTelegraphArticles(keyword string, tagID int64, order string, watchOnly int) ([]models.TelegraphArticleItem, error) {
	var articles []models.TelegraphArticleItem
	var err error

	orderBy := "a.id DESC"
	if order == "score_desc" {
		orderBy = "COALESCE(tm.importance_score, 0) DESC, a.id DESC"
	}
	if order == "watch_first" {
		orderBy = "COALESCE(wh.watch_matched, 0) DESC, COALESCE(tm.importance_score, 0) DESC, a.id DESC"
	}
	watchFilter := ""
	if watchOnly == 1 {
		watchFilter = " AND COALESCE(wh.watch_matched, 0) > 0"
	}

	if tagID > 0 && keyword != "" {
		q := "%" + keyword + "%"
		err = db.DB.Select(&articles, fmt.Sprintf(`
			SELECT
				a.id, a.title, a.source, a.status, a.created_at, a.analyzed_at,
				COALESCE(tm.importance_score, 0) AS importance_score,
				COALESCE(tm.impact_direction, '中性') AS impact_direction,
				COALESCE(tm.impact_level, '低') AS impact_level,
				COALESCE(wh.watch_matched, 0) AS watch_matched
			FROM articles a
			JOIN article_tags at ON a.id = at.article_id
			LEFT JOIN telegraph_meta tm ON a.id = tm.article_id
			LEFT JOIN (
				SELECT article_id, COUNT(*) AS watch_matched
				FROM telegraph_watch_hits
				GROUP BY article_id
			) wh ON a.id = wh.article_id
			WHERE a.source LIKE ? AND at.tag_id=? AND (a.title LIKE ? OR a.content LIKE ?) %s
			ORDER BY %s
		`, watchFilter, orderBy), telegraphSourcePrefixLike, tagID, q, q)
	} else if tagID > 0 {
		err = db.DB.Select(&articles, fmt.Sprintf(`
			SELECT
				a.id, a.title, a.source, a.status, a.created_at, a.analyzed_at,
				COALESCE(tm.importance_score, 0) AS importance_score,
				COALESCE(tm.impact_direction, '中性') AS impact_direction,
				COALESCE(tm.impact_level, '低') AS impact_level,
				COALESCE(wh.watch_matched, 0) AS watch_matched
			FROM articles a
			JOIN article_tags at ON a.id = at.article_id
			LEFT JOIN telegraph_meta tm ON a.id = tm.article_id
			LEFT JOIN (
				SELECT article_id, COUNT(*) AS watch_matched
				FROM telegraph_watch_hits
				GROUP BY article_id
			) wh ON a.id = wh.article_id
			WHERE a.source LIKE ? AND at.tag_id=? %s
			ORDER BY %s
		`, watchFilter, orderBy), telegraphSourcePrefixLike, tagID)
	} else if keyword != "" {
		q := "%" + keyword + "%"
		err = db.DB.Select(&articles, fmt.Sprintf(`
			SELECT
				a.id, a.title, a.source, a.status, a.created_at, a.analyzed_at,
				COALESCE(tm.importance_score, 0) AS importance_score,
				COALESCE(tm.impact_direction, '中性') AS impact_direction,
				COALESCE(tm.impact_level, '低') AS impact_level,
				COALESCE(wh.watch_matched, 0) AS watch_matched
			FROM articles a
			LEFT JOIN telegraph_meta tm ON a.id = tm.article_id
			LEFT JOIN (
				SELECT article_id, COUNT(*) AS watch_matched
				FROM telegraph_watch_hits
				GROUP BY article_id
			) wh ON a.id = wh.article_id
			WHERE a.source LIKE ? AND (a.title LIKE ? OR a.content LIKE ?) %s
			ORDER BY %s
		`, watchFilter, orderBy), telegraphSourcePrefixLike, q, q)
	} else {
		err = db.DB.Select(&articles, fmt.Sprintf(`
			SELECT
				a.id, a.title, a.source, a.status, a.created_at, a.analyzed_at,
				COALESCE(tm.importance_score, 0) AS importance_score,
				COALESCE(tm.impact_direction, '中性') AS impact_direction,
				COALESCE(tm.impact_level, '低') AS impact_level,
				COALESCE(wh.watch_matched, 0) AS watch_matched
			FROM articles a
			LEFT JOIN telegraph_meta tm ON a.id = tm.article_id
			LEFT JOIN (
				SELECT article_id, COUNT(*) AS watch_matched
				FROM telegraph_watch_hits
				GROUP BY article_id
			) wh ON a.id = wh.article_id
			WHERE a.source LIKE ? %s
			ORDER BY %s
		`, watchFilter, orderBy), telegraphSourcePrefixLike)
	}
	if err != nil {
		return nil, err
	}
	if err := loadTelegraphItemTags(&articles); err != nil {
		return nil, err
	}
	if err := loadTelegraphWatchMatches(&articles); err != nil {
		return nil, err
	}
	return articles, nil
}

func GetArticle(id int64) (models.Article, error) {
	var a models.Article
	err := db.DB.Get(&a, "SELECT * FROM articles WHERE id=?", id)
	if err == nil {
		a.Tags, _ = GetArticleTags(id)
	}
	return a, err
}

func ImportFile(filePath string) (models.Article, error) {
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	ext := strings.ToLower(filepath.Ext(filePath))

	var content string
	switch {
	case isTextLikeFile(ext):
		data, err := os.ReadFile(filePath)
		if err != nil {
			return models.Article{}, err
		}
		content = string(data)
	case isMinerUSupportedFile(ext):
		parsed, err := ParseFileWithMinerU(filePath)
		if err != nil {
			return models.Article{}, fmt.Errorf("MinerU 解析失败: %w", err)
		}
		content = parsed
	default:
		return models.Article{}, errors.New("不支持的文件类型，请导入 txt/md/html/pdf/图片/doc/ppt 等格式")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Article{}, errors.New("导入内容为空")
	}

	res, err := db.DB.Exec("INSERT INTO articles(title,content) VALUES(?,?)", title, content)
	if err != nil {
		return models.Article{}, err
	}
	id, _ := res.LastInsertId()
	return GetArticle(id)
}

func isTextLikeFile(ext string) bool {
	switch ext {
	case ".txt", ".md", ".markdown", ".html", ".htm":
		return true
	default:
		return false
	}
}

func isMinerUSupportedFile(ext string) bool {
	switch ext {
	case ".pdf", ".png", ".jpg", ".jpeg", ".bmp", ".tiff", ".webp", ".doc", ".docx", ".ppt", ".pptx":
		return true
	default:
		return false
	}
}

func UpdateArticleAnalysis(id int64, analysis, promptUsed, channelUsed string) error {
	now := time.Now()
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE articles SET analysis=?,prompt_used=?,channel_used=?,status=2,analyzed_at=? WHERE id=?",
		analysis, promptUsed, channelUsed, now, id); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO analysis_history(article_id,analysis,prompt_used,channel_used) VALUES(?,?,?,?)",
		id, analysis, promptUsed, channelUsed); err != nil {
		return err
	}
	return tx.Commit()
}

func GetAnalysisHistory(articleID int64) ([]models.AnalysisHistory, error) {
	var history []models.AnalysisHistory
	err := db.DB.Select(&history, "SELECT * FROM analysis_history WHERE article_id=? ORDER BY id DESC", articleID)
	return history, err
}

func UpdateArticleStatus(id int64, status int) error {
	_, err := db.DB.Exec("UPDATE articles SET status=? WHERE id=?", status, id)
	return err
}

func DeleteArticle(id int64) error {
	_, err := db.DB.Exec("DELETE FROM articles WHERE id=?", id)
	return err
}

type articleTagRow struct {
	ArticleID int64  `db:"article_id"`
	TagID     int64  `db:"id"`
	TagName   string `db:"name"`
	TagColor  string `db:"color"`
}

func loadArticleTags(articles *[]models.Article) error {
	if len(*articles) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(*articles))
	indexByID := make(map[int64]int, len(*articles))
	for i := range *articles {
		id := (*articles)[i].ID
		ids = append(ids, id)
		indexByID[id] = i
	}

	q, args, err := sqlx.In(`
		SELECT at.article_id, t.id, t.name, t.color
		FROM article_tags at
		JOIN tags t ON t.id = at.tag_id
		WHERE at.article_id IN (?)
		ORDER BY at.article_id, t.name
	`, ids)
	if err != nil {
		return err
	}

	var rows []articleTagRow
	if err := db.DB.Select(&rows, db.DB.Rebind(q), args...); err != nil {
		return err
	}

	for _, row := range rows {
		if idx, ok := indexByID[row.ArticleID]; ok {
			(*articles)[idx].Tags = append((*articles)[idx].Tags, models.Tag{
				ID:    row.TagID,
				Name:  row.TagName,
				Color: row.TagColor,
			})
		}
	}
	return nil
}

func loadTelegraphItemTags(items *[]models.TelegraphArticleItem) error {
	if len(*items) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(*items))
	indexByID := make(map[int64]int, len(*items))
	for i := range *items {
		id := (*items)[i].ID
		ids = append(ids, id)
		indexByID[id] = i
	}

	q, args, err := sqlx.In(`
		SELECT at.article_id, t.id, t.name, t.color
		FROM article_tags at
		JOIN tags t ON t.id = at.tag_id
		WHERE at.article_id IN (?)
		ORDER BY at.article_id, t.name
	`, ids)
	if err != nil {
		return err
	}

	var rows []articleTagRow
	if err := db.DB.Select(&rows, db.DB.Rebind(q), args...); err != nil {
		return err
	}

	for _, row := range rows {
		if idx, ok := indexByID[row.ArticleID]; ok {
			(*items)[idx].Tags = append((*items)[idx].Tags, models.Tag{
				ID:    row.TagID,
				Name:  row.TagName,
				Color: row.TagColor,
			})
		}
	}
	return nil
}

type telegraphWatchHitRow struct {
	ArticleID int64  `db:"article_id"`
	Code      string `db:"stock_code"`
	Name      string `db:"stock_name"`
}

func loadTelegraphWatchMatches(items *[]models.TelegraphArticleItem) error {
	if len(*items) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(*items))
	indexByID := make(map[int64]int, len(*items))
	for i := range *items {
		id := (*items)[i].ID
		ids = append(ids, id)
		indexByID[id] = i
	}

	q, args, err := sqlx.In(`
		SELECT article_id, stock_code, stock_name
		FROM telegraph_watch_hits
		WHERE article_id IN (?)
		ORDER BY article_id, stock_code
	`, ids)
	if err != nil {
		return err
	}

	var rows []telegraphWatchHitRow
	if err := db.DB.Select(&rows, db.DB.Rebind(q), args...); err != nil {
		return err
	}

	for _, row := range rows {
		if idx, ok := indexByID[row.ArticleID]; ok {
			(*items)[idx].WatchMatches = append((*items)[idx].WatchMatches, models.TelegraphWatchMatch{
				Code: row.Code,
				Name: row.Name,
			})
		}
	}
	return nil
}
