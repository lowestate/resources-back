package ds

type Material struct {
	ID       uint `gorm:"primarykey"`
	Name     string
	Quantity uint
	Place    string
}
