package nga

import "time"

type Topic struct {
	ID       int64     `json:"id"`
	FID      int64     `json:"fid"`
	Title    string    `json:"title"`
	Author   string    `json:"author"`
	AuthorID int64     `json:"authorId"`
	Replies  int       `json:"replies"`
	PostDate time.Time `json:"postDate"`
	LastPost time.Time `json:"lastPost"`
}

type Post struct {
	ID       int64
	Floor    int
	AuthorID int64
	Author   string
	Content  string
	PostDate time.Time
}

type TopicPage struct {
	Topics     []Topic
	Page       int
	TotalPages int
	HasNext    bool
	BoardName  string
}

type ThreadPage struct {
	Topic        Topic
	Posts        []Post
	Page         int
	TotalPages   int
	TotalRows    int
	MainAuthorID int64
}

type APIError struct {
	Kind, Message string
	Status        int
}

func (e *APIError) Error() string { return e.Message }
