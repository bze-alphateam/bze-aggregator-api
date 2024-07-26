package dto

import "time"

type Article struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	PictureURL  string    `json:"picture_url"`
	Description string    `json:"description"`
	PublishDate time.Time `json:"publish_date"`
	AuthorName  string    `json:"author_name"`
}
