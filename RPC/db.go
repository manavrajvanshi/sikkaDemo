package main

// Sample for mock DB data fetching.
// We can use a real database here.
func getData(user string) (greeting string, exist bool) {
	exist = true
	greeting = "Hello " + user
	return
}
