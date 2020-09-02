package lcache

import "errors"

var refreshErrEntryNotFound = errors.New("entry not found")
var refreshErrNotEnabled = errors.New("cache is not configured with graceful refresh")
var refreshErrNoLoader = errors.New("cache is not configured with a loader")
