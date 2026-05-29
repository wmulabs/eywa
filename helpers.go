package eywa

import (
	"github.com/wmulabs/eywa/internal/helpers"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
)

var (
	// GenerateID returns a new random unique identifier string.
	GenerateID = helpers.GenerateRandomID

	// NowUTC returns the current time in UTC.
	NowUTC = helpers.NowUTC

	// TodayUTC returns today's date at midnight UTC.
	TodayUTC = helpers.TodayUTC

	// NormalizePhone strips formatting and returns an E.164 phone number string.
	NormalizePhone = helpers.NormalizePhone

	// ObfuscatePhone returns a partially masked phone number for safe logging.
	ObfuscatePhone = helpers.ObfuscatePhone

	// FromHTTPStatus maps an HTTP status code to a ResponseStatus.
	FromHTTPStatus = helpers.FromHTTPStatus

	// FromHTTPResponse maps an HTTP response body and status to an ActionError or nil.
	FromHTTPResponse = helpers.FromHTTPResponse

	// GetString extracts a string value from a map[string]any by key.
	GetString = helpers.GetString

	// GetStringOrDefault extracts a string from args, returning def if missing or wrong type.
	GetStringOrDefault = helpers.GetStringOrDefault

	// GetIntOrDefault extracts an int from args, returning def if missing or wrong type.
	GetIntOrDefault = helpers.GetIntOrDefault

	// GetFloat64OrDefault extracts a float64 from args, returning def if missing or wrong type.
	GetFloat64OrDefault = helpers.GetFloat64OrDefault

	// GetMap extracts a nested map[string]any from args by key.
	GetMap = helpers.GetMap

	// GetMapOrDefault extracts a nested map from args, returning def if missing or wrong type.
	GetMapOrDefault = helpers.GetMapOrDefault

	// GetSlice extracts a []any from args by key.
	GetSlice = helpers.GetSlice

	// GetSliceOrDefault extracts a slice from args, returning def if missing or wrong type.
	GetSliceOrDefault = helpers.GetSliceOrDefault

	// GetStringSlice extracts a []string from args by key, filtering non-string elements.
	GetStringSlice = helpers.GetStringSlice

	// GetFirstMapFromSlice returns the first map element from a slice value in args.
	GetFirstMapFromSlice = helpers.GetFirstMapFromSlice

	// CombineTextPartsMultiLine joins multiple text parts with newline separators.
	CombineTextPartsMultiLine = helpers.CombineTextPartsMultiLine

	// StringMapKeys returns the sorted keys of a map[string]string.
	StringMapKeys = helpers.StringMapKeys
)

var (
	// GetLogger returns the shared application logger, creating one if needed.
	GetLogger = dbg.GetLogger

	// CreateLogger creates a new named logger instance.
	CreateLogger = dbg.CreateLogger
)
