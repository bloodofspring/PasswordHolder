package models

type Secrets struct {
	ID        int64  `pg:"id,pk"`
	CreatedAt int64 `pg:",default:extract(epoch from now())"`
	UpdatedAt int64 `pg:",default:extract(epoch from now())"`

	UserID    int64  `pg:"user_id"`
	User      *Users `pg:"rel:has-one,fk:user_id"`

	Title     string `pg:"title"`
	Login     string `pg:"login"`
	Password  string `pg:"password"`
}
