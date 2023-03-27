package dgc

type Category struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Commands    []*Command `json:"-"`
}
