package models

type Sessions struct {
	ID        int64  `pg:"id,pk"`
	CreatedAt int64 `pg:",default:extract(epoch from now())"`
	UpdatedAt int64 `pg:",default:extract(epoch from now())"`

	UserID    int64  `pg:"user_id"`
	User      *Users `pg:"rel:has-one,fk:user_id"`

	Password string `pg:"password"`
	ResetTimeInterval int64 `pg:"reset_time_interval,default:10"`
}
