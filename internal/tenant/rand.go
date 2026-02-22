package tenant

import "crypto/rand"

// randRead is a var so it can be replaced in tests.
var randRead = rand.Read
