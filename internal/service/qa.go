package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"stock-report-analysis/internal/db"
	"stock-report-analysis/internal/models"
)

type QAStreamCallbacks struct {
	OnJobStart  func(sessionID int64, questionMessageID int64, roleCount int)
	OnRoleStart func(msg models.QAMessage, role models.Role)
	OnRoleChunk func(messageID int64, roleID int64, roleName string, chunk string)
	OnRoleDone  func(msg models.QAMessage)
	OnRoleError func(messageID int64, roleID int64, roleName string, errMsg string)
	OnJobDone   func(sessionID int64)
}

type articleChunk struct {
	Index int
	Text  string
	Score int
}

var keywordPattern = regexp.MustCompile(`[\p{Han}\p{L}\p{N}]{2,}`)

const qaRoleTimeout = 90 * time.Second

func GetQASessions(articleID int64) ([]models.QASession, error) {
	var sessions []models.QASession
	err := db.DB.Select(&sessions, `
		SELECT *
		FROM qa_sessions
		WHERE article_id=?
		ORDER BY updated_at DESC, id DESC
	`, articleID)
	return sessions, err
}

func CreateQASession(articleID int64, title string) (models.QASession, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "问答会话"
	}
	res, err := db.DB.Exec(`
		INSERT INTO qa_sessions(article_id, title, summary)
		VALUES(?,?,?)
	`, articleID, trimToRunes(title, 64), "")
	if err != nil {
		return models.QASession{}, err
	}
	id, _ := res.LastInsertId()
	var session models.QASession
	err = db.DB.Get(&session, "SELECT * FROM qa_sessions WHERE id=?", id)
	return session, err
}

func RenameQASession(id int64, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("会话标题不能为空")
	}
	_, err := db.DB.Exec(`
		UPDATE qa_sessions
		SET title=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, trimToRunes(title, 64), id)
	return err
}

func DeleteQASession(id int64) error {
	_, err := db.DB.Exec("DELETE FROM qa_sessions WHERE id=?", id)
	return err
}

func GetQAPins(sessionID int64) ([]models.QAPin, error) {
	var pins []models.QAPin
	err := db.DB.Select(&pins, `
		SELECT *
		FROM qa_pins
		WHERE session_id=?
		ORDER BY id DESC
	`, sessionID)
	return pins, err
}

func SaveQAPin(pin models.QAPin) (models.QAPin, error) {
	pin.Content = strings.TrimSpace(pin.Content)
	if pin.SessionID <= 0 {
		return models.QAPin{}, errors.New("会话 ID 无效")
	}
	if pin.ArticleID <= 0 {
		return models.QAPin{}, errors.New("文章 ID 无效")
	}
	if pin.Content == "" {
		return models.QAPin{}, errors.New("记忆内容不能为空")
	}
	pin.Content = trimToRunes(pin.Content, 1200)

	if pin.ID > 0 {
		_, err := db.DB.Exec(`
			UPDATE qa_pins
			SET content=?, source_message_id=?, updated_at=CURRENT_TIMESTAMP
			WHERE id=? AND session_id=?
		`, pin.Content, pin.SourceMessageID, pin.ID, pin.SessionID)
		if err != nil {
			return models.QAPin{}, err
		}
		var updated models.QAPin
		if err := db.DB.Get(&updated, "SELECT * FROM qa_pins WHERE id=?", pin.ID); err != nil {
			return models.QAPin{}, err
		}
		return updated, nil
	}

	res, err := db.DB.Exec(`
		INSERT INTO qa_pins(session_id, article_id, source_message_id, content)
		VALUES(?,?,?,?)
	`, pin.SessionID, pin.ArticleID, pin.SourceMessageID, pin.Content)
	if err != nil {
		return models.QAPin{}, err
	}
	id, _ := res.LastInsertId()
	var created models.QAPin
	err = db.DB.Get(&created, "SELECT * FROM qa_pins WHERE id=?", id)
	return created, err
}

func DeleteQAPin(id int64) error {
	_, err := db.DB.Exec("DELETE FROM qa_pins WHERE id=?", id)
	return err
}

func GetQAMessages(sessionID int64) ([]models.QAMessage, error) {
	var messages []models.QAMessage
	err := db.DB.Select(&messages, `
		SELECT
			m.*,
			COALESCE(r.name, '') AS role_name
		FROM qa_messages m
		LEFT JOIN roles r ON r.id = m.role_id
		WHERE m.session_id=?
		ORDER BY m.id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return messages, nil
	}

	var evidences []models.QAEvidence
	err = db.DB.Select(&evidences, `
		SELECT e.*
		FROM qa_evidences e
		JOIN qa_messages m ON m.id = e.message_id
		WHERE m.session_id=?
		ORDER BY e.id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}

	byMessage := make(map[int64][]models.QAEvidence)
	for _, ev := range evidences {
		byMessage[ev.MessageID] = append(byMessage[ev.MessageID], ev)
	}
	for i := range messages {
		messages[i].Evidences = byMessage[messages[i].ID]
	}
	return messages, nil
}

func AskQuestion(sessionID int64, articleID int64, question string, cb QAStreamCallbacks) (int64, error) {
	return AskQuestionWithContextAndFollowUp(context.Background(), sessionID, articleID, question, 0, cb)
}

func AskQuestionWithContext(ctx context.Context, sessionID int64, articleID int64, question string, cb QAStreamCallbacks) (int64, error) {
	return AskQuestionWithContextAndFollowUp(ctx, sessionID, articleID, question, 0, cb)
}

func AskQuestionWithContextAndFollowUp(ctx context.Context, sessionID int64, articleID int64, question string, followUpMessageID int64, cb QAStreamCallbacks) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	question = strings.TrimSpace(question)
	if question == "" {
		return 0, errors.New("问题不能为空")
	}
	if followUpMessageID > 0 && sessionID <= 0 {
		return 0, errors.New("继续追问需要在已有会话中进行")
	}
	log.Printf("[QA] ask start session=%d article=%d question=%q", sessionID, articleID, trimToRunes(question, 80))

	if sessionID == 0 {
		session, err := CreateQASession(articleID, question)
		if err != nil {
			return 0, err
		}
		sessionID = session.ID
		log.Printf("[QA] created session=%d article=%d", sessionID, articleID)
	}

	roles, cleanedQuestion, err := ResolveRolesByMentions(question)
	if err != nil {
		return 0, err
	}
	log.Printf("[QA] resolved roles session=%d count=%d question=%q", sessionID, len(roles), trimToRunes(cleanedQuestion, 80))

	followUpContext := ""
	if followUpMessageID > 0 {
		followUpContext, err = buildFollowUpContext(sessionID, articleID, followUpMessageID)
		if err != nil {
			return 0, err
		}
	}

	userMessageID, err := insertQAMessage(models.QAMessage{
		SessionID:   sessionID,
		ArticleID:   articleID,
		RoleType:    "user",
		RoleID:      0,
		Content:     cleanedQuestion,
		Status:      "done",
		ParentID:    followUpMessageID,
		CreatedAt:   time.Now(),
		RoleName:    "",
		Evidences:   nil,
		ErrorReason: "",
	})
	if err != nil {
		return 0, err
	}

	if cb.OnJobStart != nil {
		cb.OnJobStart(sessionID, userMessageID, len(roles))
	}
	log.Printf("[QA] job started session=%d user_message=%d", sessionID, userMessageID)
	if err := ctx.Err(); err != nil {
		if cb.OnJobDone != nil {
			cb.OnJobDone(sessionID)
		}
		return userMessageID, nil
	}

	article, err := GetArticle(articleID)
	if err != nil {
		return userMessageID, err
	}
	chunks := buildArticleChunks(article.Content, 900)
	retrieved := retrieveTopChunks(cleanedQuestion, chunks, 6)

	summary, _ := getSessionSummary(sessionID)
	pins, _ := getSessionPins(sessionID)
	channel, err := getDefaultChannel()
	if err != nil {
		return userMessageID, err
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 2)

	answerSummaries := make([]string, 0, len(roles))
	var ansMu sync.Mutex

	for _, role := range roles {
		role := role
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}
			log.Printf("[QA] role begin session=%d role=%d(%s)", sessionID, role.ID, role.Name)

			assistantMessageID, err := insertQAMessage(models.QAMessage{
				SessionID: sessionID,
				ArticleID: articleID,
				RoleType:  "assistant",
				RoleID:    role.ID,
				Content:   "",
				Status:    "running",
				ParentID:  userMessageID,
			})
			if err != nil {
				if cb.OnRoleError != nil {
					cb.OnRoleError(0, role.ID, role.Name, err.Error())
				}
				return
			}

			if cb.OnRoleStart != nil {
				cb.OnRoleStart(models.QAMessage{ID: assistantMessageID, SessionID: sessionID, ArticleID: articleID, ParentID: userMessageID, RoleType: "assistant", RoleID: role.ID, RoleName: role.Name, Status: "running"}, role)
			}

			activeChannel := channel
			if role.ModelOverride != "" {
				activeChannel.Model = role.ModelOverride
			}

			prompt := buildQASystemPrompt(role)
			qaInput := buildQAInput(summary, pins, followUpContext, cleanedQuestion, retrieved)
			startedAt := time.Now()
			roleCtx, cancelRole := context.WithTimeout(ctx, qaRoleTimeout)
			result, err := AnalyzeArticleDetailedWithContext(roleCtx, activeChannel, prompt, qaInput, AnalysisModeText, func(chunk string) {
				if cb.OnRoleChunk != nil {
					cb.OnRoleChunk(assistantMessageID, role.ID, role.Name, chunk)
				}
			})
			cancelRole()
			if err != nil {
				if errors.Is(roleCtx.Err(), context.DeadlineExceeded) {
					err = fmt.Errorf("角色回答超时（%d 秒）", int(qaRoleTimeout.Seconds()))
				} else if errors.Is(roleCtx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
					err = errors.New("已取消本次提问")
				}
				errMsg := err.Error()
				log.Printf("[QA] role failed session=%d role=%d(%s) message=%d err=%s", sessionID, role.ID, role.Name, assistantMessageID, errMsg)
				_ = updateQAMessageFailure(assistantMessageID, errMsg)
				_ = insertQARun(models.QARun{
					SessionID:        sessionID,
					MessageID:        assistantMessageID,
					ArticleID:        articleID,
					RoleID:           role.ID,
					RoleName:         role.Name,
					Success:          0,
					ErrorReason:      classifyErrorReason(err),
					DurationMs:       time.Since(startedAt).Milliseconds(),
					PromptTokens:     result.PromptTokens,
					CompletionTokens: result.CompletionTokens,
					TotalTokens:      result.TotalTokens,
				})
				if cb.OnRoleError != nil {
					cb.OnRoleError(assistantMessageID, role.ID, role.Name, errMsg)
				}
				return
			}

			_ = updateQAMessageSuccess(assistantMessageID, result)
			_ = insertQARun(models.QARun{
				SessionID:        sessionID,
				MessageID:        assistantMessageID,
				ArticleID:        articleID,
				RoleID:           role.ID,
				RoleName:         role.Name,
				Success:          1,
				ErrorReason:      "",
				DurationMs:       result.DurationMs,
				PromptTokens:     result.PromptTokens,
				CompletionTokens: result.CompletionTokens,
				TotalTokens:      result.TotalTokens,
			})
			_ = saveEvidences(assistantMessageID, retrieved)
			log.Printf("[QA] role done session=%d role=%d(%s) message=%d duration_ms=%d", sessionID, role.ID, role.Name, assistantMessageID, result.DurationMs)

			ansMu.Lock()
			answerSummaries = append(answerSummaries, fmt.Sprintf("A[%s]: %s", role.Name, trimToRunes(result.Text, 240)))
			ansMu.Unlock()

			if cb.OnRoleDone != nil {
				cb.OnRoleDone(models.QAMessage{
					ID:               assistantMessageID,
					SessionID:        sessionID,
					ArticleID:        articleID,
					ParentID:         userMessageID,
					RoleType:         "assistant",
					RoleID:           role.ID,
					RoleName:         role.Name,
					Content:          result.Text,
					Status:           "done",
					DurationMs:       result.DurationMs,
					PromptTokens:     result.PromptTokens,
					CompletionTokens: result.CompletionTokens,
					TotalTokens:      result.TotalTokens,
				})
			}
		}()
	}

	wg.Wait()
	log.Printf("[QA] all roles done session=%d", sessionID)

	if ctx.Err() == nil {
		summaryPayload := fmt.Sprintf("Q: %s\n%s", trimToRunes(cleanedQuestion, 240), strings.Join(answerSummaries, "\n"))
		_ = appendSessionSummary(sessionID, summaryPayload)
	}
	if cb.OnJobDone != nil {
		cb.OnJobDone(sessionID)
	}
	log.Printf("[QA] job done session=%d", sessionID)
	return userMessageID, nil
}

func getDefaultChannel() (models.AIChannel, error) {
	channels, err := GetChannels()
	if err != nil {
		return models.AIChannel{}, err
	}
	if len(channels) == 0 {
		return models.AIChannel{}, errors.New("请先在设置里配置至少一个 AI 渠道")
	}
	for _, ch := range channels {
		if ch.IsDefault == 1 {
			return ch, nil
		}
	}
	return channels[0], nil
}

func insertQAMessage(msg models.QAMessage) (int64, error) {
	res, err := db.DB.Exec(`
		INSERT INTO qa_messages(
			session_id, article_id, parent_id, role_type, role_id, content, status, error_reason,
			duration_ms, prompt_tokens, completion_tokens, total_tokens
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		msg.SessionID,
		msg.ArticleID,
		msg.ParentID,
		msg.RoleType,
		msg.RoleID,
		msg.Content,
		coalesceString(msg.Status, "done"),
		msg.ErrorReason,
		msg.DurationMs,
		msg.PromptTokens,
		msg.CompletionTokens,
		msg.TotalTokens,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateQAMessageSuccess(messageID int64, result AnalysisResult) error {
	_, err := db.DB.Exec(`
		UPDATE qa_messages
		SET content=?, status='done', error_reason='', duration_ms=?, prompt_tokens=?, completion_tokens=?, total_tokens=?
		WHERE id=?
	`, result.Text, result.DurationMs, result.PromptTokens, result.CompletionTokens, result.TotalTokens, messageID)
	return err
}

func updateQAMessageFailure(messageID int64, errMsg string) error {
	_, err := db.DB.Exec(`
		UPDATE qa_messages
		SET status='failed', error_reason=?
		WHERE id=?
	`, trimToRunes(errMsg, 255), messageID)
	return err
}

func saveEvidences(messageID int64, chunks []articleChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	for _, ch := range chunks {
		quote := trimToRunes(ch.Text, 180)
		if _, err := db.DB.Exec(`
			INSERT INTO qa_evidences(message_id, chunk_index, quote, reason)
			VALUES(?,?,?,?)
		`, messageID, ch.Index, quote, "问题关键词命中"); err != nil {
			return err
		}
	}
	return nil
}

func insertQARun(run models.QARun) error {
	_, err := db.DB.Exec(`
		INSERT INTO qa_runs(
			session_id, message_id, article_id, role_id, role_name, success, error_reason,
			duration_ms, prompt_tokens, completion_tokens, total_tokens
		) VALUES(?,?,?,?,?,?,?,?,?,?,?)
	`,
		run.SessionID,
		run.MessageID,
		run.ArticleID,
		run.RoleID,
		run.RoleName,
		run.Success,
		run.ErrorReason,
		run.DurationMs,
		run.PromptTokens,
		run.CompletionTokens,
		run.TotalTokens,
	)
	return err
}

func getSessionSummary(sessionID int64) (string, error) {
	var summary sql.NullString
	err := db.DB.Get(&summary, "SELECT summary FROM qa_sessions WHERE id=?", sessionID)
	if err != nil {
		return "", err
	}
	if !summary.Valid {
		return "", nil
	}
	return summary.String, nil
}

func appendSessionSummary(sessionID int64, appendText string) error {
	current, _ := getSessionSummary(sessionID)
	next := strings.TrimSpace(strings.TrimSpace(current) + "\n" + strings.TrimSpace(appendText))
	next = tailRunes(next, 2000)
	_, err := db.DB.Exec(`
		UPDATE qa_sessions
		SET summary=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, next, sessionID)
	return err
}

func buildQASystemPrompt(role models.Role) string {
	base := strings.TrimSpace(role.SystemPrompt)
	if base == "" {
		base = fallbackRolePrompt
	}
	return base + `

你正在回答用户对报告的追问。要求：
1) 只基于提供的报告上下文回答，不得编造。
2) 输出纯文本，不要 JSON。
3) 结尾单独一行写“参考片段: x,y,z”（x/y/z 为片段编号）。`
}

func buildQAInput(summary string, pins []models.QAPin, followUpContext string, question string, chunks []articleChunk) string {
	var b strings.Builder
	if strings.TrimSpace(summary) != "" {
		b.WriteString("会话摘要:\n")
		b.WriteString(summary)
		b.WriteString("\n\n")
	}
	if len(pins) > 0 {
		b.WriteString("固定记忆(用户确认事实):\n")
		for i, pin := range pins {
			b.WriteString(fmt.Sprintf("(P%d) %s\n", i+1, pin.Content))
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(followUpContext) != "" {
		b.WriteString("上轮回答上下文(继续追问):\n")
		b.WriteString(followUpContext)
		b.WriteString("\n\n")
	}
	b.WriteString("报告相关片段:\n")
	for _, ch := range chunks {
		b.WriteString(fmt.Sprintf("[%d] %s\n", ch.Index, ch.Text))
	}
	b.WriteString("\n用户问题:\n")
	b.WriteString(question)
	return b.String()
}

func buildFollowUpContext(sessionID int64, articleID int64, followUpMessageID int64) (string, error) {
	var msg models.QAMessage
	if err := db.DB.Get(&msg, `
		SELECT m.*
		FROM qa_messages m
		WHERE m.id=? AND m.session_id=? AND m.article_id=? AND m.role_type='assistant'
	`, followUpMessageID, sessionID, articleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("未找到可继续追问的回答")
		}
		return "", err
	}

	var evidences []models.QAEvidence
	if err := db.DB.Select(&evidences, `
		SELECT *
		FROM qa_evidences
		WHERE message_id=?
		ORDER BY id ASC
		LIMIT 4
	`, followUpMessageID); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("上轮回答摘要:\n")
	b.WriteString(trimToRunes(strings.TrimSpace(msg.Content), 1200))
	if len(evidences) > 0 {
		b.WriteString("\n\n上轮回答引用片段:\n")
		for _, ev := range evidences {
			b.WriteString(fmt.Sprintf("[%d] %s\n", ev.ChunkIndex, trimToRunes(ev.Quote, 180)))
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func getSessionPins(sessionID int64) ([]models.QAPin, error) {
	var pins []models.QAPin
	err := db.DB.Select(&pins, `
		SELECT *
		FROM qa_pins
		WHERE session_id=?
		ORDER BY id DESC
	`, sessionID)
	return pins, err
}

func buildArticleChunks(content string, maxLen int) []articleChunk {
	if maxLen <= 0 {
		maxLen = 900
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	parts := strings.Split(content, "\n")

	chunks := make([]articleChunk, 0)
	var current strings.Builder
	index := 1
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text == "" {
			return
		}
		chunks = append(chunks, articleChunk{Index: index, Text: text})
		index++
		current.Reset()
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if current.Len() > 0 && utf8.RuneCountInString(current.String()+"\n"+p) > maxLen {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(p)
	}
	flush()

	if len(chunks) == 0 && strings.TrimSpace(content) != "" {
		chunks = append(chunks, articleChunk{Index: 1, Text: trimToRunes(strings.TrimSpace(content), maxLen)})
	}
	return chunks
}

func retrieveTopChunks(question string, chunks []articleChunk, k int) []articleChunk {
	if len(chunks) == 0 {
		return nil
	}
	if k <= 0 {
		k = 6
	}
	if k > len(chunks) {
		k = len(chunks)
	}

	terms := extractKeywords(question)
	for i := range chunks {
		score := 0
		textLower := strings.ToLower(chunks[i].Text)
		for _, t := range terms {
			score += strings.Count(textLower, t)
		}
		chunks[i].Score = score
	}

	sorted := make([]articleChunk, len(chunks))
	copy(sorted, chunks)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Score == sorted[j].Score {
			return sorted[i].Index < sorted[j].Index
		}
		return sorted[i].Score > sorted[j].Score
	})

	if sorted[0].Score <= 0 {
		return sorted[:k]
	}
	selected := sorted[:k]
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Index < selected[j].Index
	})
	return selected
}

func extractKeywords(question string) []string {
	question = strings.ToLower(question)
	matches := keywordPattern.FindAllString(question, -1)
	if len(matches) == 0 {
		return []string{question}
	}
	seen := make(map[string]struct{})
	keywords := make([]string, 0, len(matches))
	for _, m := range matches {
		m = strings.TrimSpace(m)
		if utf8.RuneCountInString(m) < 2 {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		keywords = append(keywords, m)
	}
	if len(keywords) == 0 {
		return []string{question}
	}
	return keywords
}

func trimToRunes(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

func tailRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[len(r)-limit:])
}

func coalesceString(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func classifyErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "已取消"), strings.Contains(msg, "canceled"), strings.Contains(msg, "cancelled"):
		return "canceled"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "timed out"):
		return "timeout"
	case strings.Contains(msg, "rate limit"), strings.Contains(msg, "too many requests"), strings.Contains(msg, "429"):
		return "rate_limit"
	case strings.Contains(msg, "unauthorized"), strings.Contains(msg, "401"), strings.Contains(msg, "forbidden"), strings.Contains(msg, "403"):
		return "auth"
	case strings.Contains(msg, "context length"), strings.Contains(msg, "token"), strings.Contains(msg, "max context"):
		return "context_limit"
	case strings.Contains(msg, "connection"), strings.Contains(msg, "network"), strings.Contains(msg, "dns"):
		return "network"
	default:
		return "unknown"
	}
}

func GetQADashboardByDays(days int) (models.QADashboard, error) {
	dashboard := models.QADashboard{}
	if days < 0 {
		days = 0
	}
	clause, args := buildTimeFilter(days)

	totals := struct {
		TotalRuns     int64         `db:"total_runs"`
		SuccessRuns   sql.NullInt64 `db:"success_runs"`
		FailedRuns    sql.NullInt64 `db:"failed_runs"`
		AvgDurationMs sql.NullInt64 `db:"avg_duration_ms"`
		TotalTokens   sql.NullInt64 `db:"total_tokens"`
		PromptTokens  sql.NullInt64 `db:"prompt_tokens"`
		OutputTokens  sql.NullInt64 `db:"output_tokens"`
	}{}
	if err := db.DB.Get(&totals, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_runs,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS success_runs,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) AS failed_runs,
			AVG(duration_ms) AS avg_duration_ms,
			SUM(total_tokens) AS total_tokens,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS output_tokens
		FROM qa_runs
		%s
	`, clause), args...); err != nil {
		return dashboard, err
	}

	dashboard.TotalRuns = int(totals.TotalRuns)
	dashboard.SuccessRuns = int(totals.SuccessRuns.Int64)
	dashboard.FailedRuns = int(totals.FailedRuns.Int64)
	dashboard.AvgDurationMs = totals.AvgDurationMs.Int64
	dashboard.TotalTokens = totals.TotalTokens.Int64
	dashboard.PromptTokens = totals.PromptTokens.Int64
	dashboard.OutputTokens = totals.OutputTokens.Int64
	dashboard.SuccessRate = percentage(dashboard.SuccessRuns, dashboard.TotalRuns)

	roleRows := []struct {
		RoleID       int64         `db:"role_id"`
		RoleName     string        `db:"role_name"`
		TotalRuns    int64         `db:"total_runs"`
		SuccessRuns  sql.NullInt64 `db:"success_runs"`
		FailedRuns   sql.NullInt64 `db:"failed_runs"`
		AvgDuration  sql.NullInt64 `db:"avg_duration"`
		TotalTokens  sql.NullInt64 `db:"total_tokens"`
		PromptTokens sql.NullInt64 `db:"prompt_tokens"`
		OutputTokens sql.NullInt64 `db:"output_tokens"`
	}{}
	if err := db.DB.Select(&roleRows, fmt.Sprintf(`
		SELECT
			role_id,
			role_name,
			COUNT(*) AS total_runs,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS success_runs,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) AS failed_runs,
			AVG(duration_ms) AS avg_duration,
			SUM(total_tokens) AS total_tokens,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS output_tokens
		FROM qa_runs
		%s
		GROUP BY role_id, role_name
		ORDER BY total_runs DESC, role_name ASC
	`, clause), args...); err != nil {
		return dashboard, err
	}

	dashboard.ByRole = make([]models.QARoleMetric, 0, len(roleRows))
	for _, row := range roleRows {
		totalRuns := int(row.TotalRuns)
		successRuns := int(row.SuccessRuns.Int64)
		dashboard.ByRole = append(dashboard.ByRole, models.QARoleMetric{
			RoleID:       row.RoleID,
			RoleName:     row.RoleName,
			TotalRuns:    totalRuns,
			SuccessRuns:  successRuns,
			FailedRuns:   int(row.FailedRuns.Int64),
			SuccessRate:  percentage(successRuns, totalRuns),
			AvgDuration:  row.AvgDuration.Int64,
			TotalTokens:  row.TotalTokens.Int64,
			PromptTokens: row.PromptTokens.Int64,
			OutputTokens: row.OutputTokens.Int64,
		})
	}

	reasonClause, reasonArgs := buildFailureFilter(days)
	reasonRows := []struct {
		Reason string `db:"reason"`
		Count  int64  `db:"count"`
	}{}
	if err := db.DB.Select(&reasonRows, fmt.Sprintf(`
		SELECT error_reason AS reason, COUNT(*) AS count
		FROM qa_runs
		%s
		GROUP BY error_reason
		ORDER BY count DESC, reason ASC
		LIMIT 8
	`, reasonClause), reasonArgs...); err != nil {
		return dashboard, err
	}

	dashboard.FailureTop = make([]models.FailureReasonMetric, 0, len(reasonRows))
	for _, row := range reasonRows {
		dashboard.FailureTop = append(dashboard.FailureTop, models.FailureReasonMetric{Reason: row.Reason, Count: int(row.Count)})
	}

	return dashboard, nil
}
