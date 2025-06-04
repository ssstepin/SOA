package repository

import (
	"fmt"
	"github.com/Masterminds/squirrel"
	"log"
	"posts/model"
	"posts/storage"
)

type PostsRepository struct {
	postgres storage.Postgres
	sb       squirrel.StatementBuilderType
}

func NewPostsRepository(postgres storage.Postgres) *PostsRepository {
	return &PostsRepository{
		postgres: postgres,
		sb:       squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (s *PostsRepository) CreatePost(post *model.Post) error {
	query, args, err := s.sb.Insert("posts").
		Columns("user_id", "text").
		Values(post.UserId, post.Text).
		Suffix("RETURNING id, created_at, updated_at").
		ToSql()

	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	err = s.postgres.DB().QueryRow(query, args...).Scan(
		&post.ID,
		&post.CreatedAt,
		&post.UpdatedAt,
	)

	if err != nil {

		return fmt.Errorf("failed to create post: %w", err)
	}

	log.Printf("Created post: %v", post)

	return nil
}
