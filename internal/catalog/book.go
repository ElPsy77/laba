// Package catalog is the counterpart of kz.iitu.springlab.catalog:
// the domain type, the in-memory repository and the service (Listing 1).
package catalog

// Book — `public record Book(Long id, String title, String author, int year)`.
// A Go struct passed by value is just as immutable as a record, and the JSON
// tags play the role of Jackson. ID 0 stands for Java's null: "not saved yet".
type Book struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
}
