package main

// Book is one book the site presents: the data the book layout and the
// books page draw from.
type Book struct {
	ID          string // the front matter's bookid; empty for the first book
	Title       string
	Subtitle    string
	URL         string
	CoverPrefix string // static/img/PREFIX-{360,540,720,1080}.{avif,webp} and PREFIX-720.jpg
	CoverAlt    string
	Hook        string
	Line        string // one line for the books page
	Ledes       []string
	Status      string // empty once the book is out; shown in place of the buy button
	Progress    int    // while in progress: the author's estimate of how much is done, in percent
}

// Released reports whether the book can be bought.
func (b *Book) Released() bool { return b.Status == "" }

var books = []*Book{
	{
		Title:       "Building AI Agents in Go",
		Subtitle:    "From the agent loop to production",
		URL:         "/book/",
		CoverPrefix: "cover",
		CoverAlt:    "The cover of the book: a small blue gopher holding a clipboard stands in a pool of amber light on a stone plaza at night, while a towering stone android kneels and reaches toward it with an open hand.",
		Hook:        "An agent is a model in a loop. This book has you build the loop.",
		Line:        "Build an AI agent in Go from the first request to a service you can leave running, with the standard library. For Go developers new to language models.",
	},
	{
		ID:          "production",
		Title:       "Running Go Agents in Production",
		Subtitle:    "Identity, governance, and cost for agents at scale",
		URL:         "/books/running-go-agents-in-production/",
		CoverPrefix: "cover2",
		CoverAlt:    "The cover of Running Go Agents in Production: a small blue gopher with a clipboard stands at a lit control desk with a lever and green gauges, while a stone android kneels beside it with an open hand, under a vaulted hall of cables and screens.",
		Hook:        "Every call an agent makes asks five questions. This book builds the platform that answers them.",
		Line:        "Run agents for a company: every call says who it acts for, rules decide what an agent may do, and cost and state stay bounded as the platform grows.",
		Ledes: []string{
			"Run Go agents for a company: many people, teams, and agents on one platform, every call saying who it acts for, and rules, not the model, deciding what an agent may do.",
			"It picks up where the first book ends, with an agent that works, and builds what it takes to run agents for other people.",
		},
		Status:   "In progress",
		Progress: 50,
	},
}

// bookByID is the book with the front matter id, the first book for "".
func bookByID(id string) *Book {
	for _, b := range books {
		if b.ID == id {
			return b
		}
	}
	panic("unknown bookid " + id)
}
