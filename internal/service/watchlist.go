package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

const telegraphWatchlistConfigKey = "telegraph_watchlist_v1"

var onlyDigitRegexp = regexp.MustCompile(`\D+`)

func GetTelegraphWatchlist() ([]models.WatchStock, error) {
	var raw string
	err := db.DB.Get(&raw, "SELECT value FROM app_configs WHERE key=?", telegraphWatchlistConfigKey)
	if errors.Is(err, sql.ErrNoRows) {
		return []models.WatchStock{}, nil
	}
	if err != nil {
		return nil, err
	}

	var items []models.WatchStock
	if json.Unmarshal([]byte(raw), &items) != nil {
		return []models.WatchStock{}, nil
	}
	return normalizeWatchStocks(items), nil
}

func SaveTelegraphWatchlist(items []models.WatchStock) error {
	items = normalizeWatchStocks(items)
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`
		INSERT INTO app_configs(key, value, updated_at)
		VALUES(?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
	`, telegraphWatchlistConfigKey, string(data))
	return err
}

func normalizeWatchStocks(items []models.WatchStock) []models.WatchStock {
	if len(items) == 0 {
		return []models.WatchStock{}
	}
	out := make([]models.WatchStock, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		code := onlyDigitRegexp.ReplaceAllString(strings.TrimSpace(item.Code), "")
		if len(code) != 6 {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}

		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = code
		}

		aliases := make([]string, 0, len(item.Aliases))
		aliasSeen := map[string]struct{}{}
		for _, alias := range item.Aliases {
			a := strings.TrimSpace(alias)
			if a == "" {
				continue
			}
			if a == name || a == code {
				continue
			}
			if _, ok := aliasSeen[a]; ok {
				continue
			}
			aliasSeen[a] = struct{}{}
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		out = append(out, models.WatchStock{
			Code:    code,
			Name:    name,
			Aliases: aliases,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Code < out[j].Code
	})
	return out
}

func RefreshTelegraphWatchHits(articleID int64, title string, content string) error {
	watchlist, err := GetTelegraphWatchlist()
	if err != nil {
		return err
	}
	return refreshTelegraphWatchHitsWithList(articleID, title, content, watchlist)
}

func RebuildTelegraphWatchHits() error {
	watchlist, err := GetTelegraphWatchlist()
	if err != nil {
		return err
	}
	rows := []struct {
		ID      int64  `db:"id"`
		Title   string `db:"title"`
		Content string `db:"content"`
	}{}
	if err := db.DB.Select(&rows, "SELECT id,title,content FROM articles WHERE source LIKE ? ORDER BY id DESC", telegraphSourcePrefixLike); err != nil {
		return err
	}
	for _, row := range rows {
		if err := refreshTelegraphWatchHitsWithList(row.ID, row.Title, row.Content, watchlist); err != nil {
			return err
		}
	}
	return nil
}

func refreshTelegraphWatchHitsWithList(articleID int64, title string, content string, watchlist []models.WatchStock) error {
	if articleID <= 0 {
		return nil
	}
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM telegraph_watch_hits WHERE article_id=?", articleID); err != nil {
		return err
	}

	matches := matchWatchStocks(title+"\n"+content, watchlist)
	for _, m := range matches {
		if _, err := tx.Exec(`
			INSERT INTO telegraph_watch_hits(article_id, stock_code, stock_name, match_type)
			VALUES(?,?,?,?)
			ON CONFLICT(article_id, stock_code) DO UPDATE SET
				stock_name=excluded.stock_name,
				match_type=excluded.match_type,
				created_at=CURRENT_TIMESTAMP
		`, articleID, m.Code, m.Name, "auto"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func matchWatchStocks(text string, watchlist []models.WatchStock) []models.TelegraphWatchMatch {
	textLower := strings.ToLower(text)
	if len(watchlist) == 0 || strings.TrimSpace(textLower) == "" {
		return nil
	}
	out := make([]models.TelegraphWatchMatch, 0, 4)
	for _, stock := range watchlist {
		if stock.Code == "" {
			continue
		}
		matched := strings.Contains(text, stock.Code)
		if !matched && stock.Name != "" {
			matched = strings.Contains(textLower, strings.ToLower(stock.Name))
		}
		if !matched {
			for _, alias := range stock.Aliases {
				if strings.Contains(textLower, strings.ToLower(alias)) {
					matched = true
					break
				}
			}
		}
		if matched {
			out = append(out, models.TelegraphWatchMatch{
				Code: stock.Code,
				Name: stock.Name,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Code < out[j].Code
	})
	return out
}
