package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"statistics/model"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) InitSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS events (
			id SERIAL PRIMARY KEY,
			event_type VARCHAR(10) NOT NULL CHECK (event_type IN ('like', 'view', 'comment')),
			user_id INTEGER NOT NULL,
			post_id INTEGER NOT NULL,
			comment_text TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create events table: %v", err)
	}

	_, err = r.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_events_post_id ON events(post_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create post_id index: %v", err)
	}

	_, err = r.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at)
	`)
	return err
}

func (r *EventRepository) SaveEvent(ctx context.Context, eventType string, userID, postID uint32, comment *string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO events (event_type, user_id, post_id, comment_text) VALUES ($1, $2, $3, $4)",
		eventType, userID, postID, comment,
	)
	return err
}

func (r *EventRepository) GetPostStats(ctx context.Context, postID uint32) (*model.PostStats, error) {
	var stats model.PostStats
	err := r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE event_type = 'like') AS likes,
			COUNT(*) FILTER (WHERE event_type = 'view') AS views,
			COUNT(*) FILTER (WHERE event_type = 'comment') AS comments
		FROM events
		WHERE post_id = $1`, postID).Scan(&stats.Likes, &stats.Views, &stats.Comments)
	return &stats, err
}

func (r *EventRepository) GetPostTrends(ctx context.Context, postID uint32, days int) ([]model.DayStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT 
			date_trunc('day', created_at) AS day,
			COUNT(*) FILTER (WHERE event_type = 'like') AS likes,
			COUNT(*) FILTER (WHERE event_type = 'view') AS views,
			COUNT(*) FILTER (WHERE event_type = 'comment') AS comments
		FROM events
		WHERE post_id = $1
		AND created_at >= NOW() - $2 * INTERVAL '1 day'
		GROUP BY day
		ORDER BY day DESC`, postID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []model.DayStats
	for rows.Next() {
		var ds model.DayStats
		if err := rows.Scan(&ds.Date, &ds.Likes, &ds.Views, &ds.Comments); err != nil {
			return nil, err
		}
		trends = append(trends, ds)
	}
	return trends, nil
}

func (r *EventRepository) GetTopPosts(ctx context.Context, by string, limit int) ([]model.PostStats, error) {
	column := map[string]string{
		"likes":    "COUNT(*) FILTER (WHERE event_type = 'like')",
		"views":    "COUNT(*) FILTER (WHERE event_type = 'view')",
		"comments": "COUNT(*) FILTER (WHERE event_type = 'comment')",
	}[by]

	query := fmt.Sprintf(`
		SELECT 
			post_id,
			%s AS count
		FROM events
		GROUP BY post_id
		ORDER BY count DESC
		LIMIT $1`, column)

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.PostStats
	for rows.Next() {
		var ps model.PostStats
		if err := rows.Scan(&ps.PostID, &ps.Count); err != nil {
			return nil, err
		}
		posts = append(posts, ps)
	}
	return posts, nil
}

func (r *EventRepository) GetTopUsers(ctx context.Context, by string, limit int) ([]model.UserStats, error) {
	column := map[string]string{
		"likes":    "COUNT(*) FILTER (WHERE event_type = 'like')",
		"views":    "COUNT(*) FILTER (WHERE event_type = 'view')",
		"comments": "COUNT(*) FILTER (WHERE event_type = 'comment')",
	}[by]

	query := fmt.Sprintf(`
		SELECT 
			user_id,
			%s AS count
		FROM events
		GROUP BY user_id
		ORDER BY count DESC
		LIMIT $1`, column)

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.UserStats
	for rows.Next() {
		var us model.UserStats
		if err := rows.Scan(&us.UserID, &us.Count); err != nil {
			return nil, err
		}
		users = append(users, us)
	}
	return users, nil
}
