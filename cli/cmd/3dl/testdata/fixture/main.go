package main

// getUserName returns the customer display name.
func getUserName(id int) string {
	sessionUser := lookup(id) // 3dl:allow user -- passport session API field
	tmp := customerEmail(id)  // 3dl:allow customer
	name := "the user id inside a string literal"
	diary := open()
	return name
}
