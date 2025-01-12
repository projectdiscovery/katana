package normalizer

// dateTimePatterns contains regex patterns for various date and time formats
// The ordering is important for proper matching
var dateTimePatterns = []string{
	/* with days */
	"[a-zA-Z]{3,} [0-9]{1,2} [a-zA-Z]{3,} [0-9]{4}",
	"[a-zA-Z]{3,} [0-9]{1,2} [a-zA-Z]{3,} '[0-9]{2}",
	"[a-zA-Z]{3,} [0-9]{1,2} [a-zA-Z]{3,}",

	/* only numeric */
	"[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}",
	"[0-9]{4}\\.[0-9]{1,2}\\.[0-9]{1,2}",
	"[0-9]{4}/[0-9]{1,2}/[0-9]{1,2}",
	"[0-9]{1,2}-[0-9]{1,2}-[0-9]{4}",
	"[0-9]{1,2}\\.[0-9]{1,2}\\.[0-9]{4}",
	"[0-9]{1,2}/[0-9]{1,2}/[0-9]{4}",
	"[0-9]{1,2}-[0-9]{1,2}-'[0-9]{2}",
	"[0-9]{1,2}\\.[0-9]{1,2}\\.'[0-9]{2}",
	"[0-9]{1,2}/[0-9]{1,2}/'[0-9]{2}",
	"[0-9]{1,2}-[0-9]{1,2}-[0-9]{2}",
	"[0-9]{1,2}\\.[0-9]{1,2}\\.[0-9]{2}",
	"[0-9]{1,2}/[0-9]{1,2}/[0-9]{2}",

	/* long months */
	"[0-9]{1,2} [a-zA-Z]{3,} [0-9]{4}",
	"[0-9]{1,2}th [a-zA-Z]{3,} [0-9]{4}",
	"[0-9]{1,2}th [a-zA-Z]{3,}",
	"[0-9]{4} [a-zA-Z]{3,} [0-9]{1,2}",
	"[0-9]{4}[a-zA-Z]{3,}[0-9]{1,2}",
	"[a-zA-Z]{3,} [0-9]{4}",
	"[a-zA-Z]{3,} '[0-9]{2}",
	"[a-zA-Z]{3,} [0-9]{1,2} [0-9]{4}",
	"[a-zA-Z]{3,} [0-9]{1,2}, [0-9]{4}",
	"[a-zA-Z]{3,} [0-9]{1,2} '[0-9]{2}",
	"[a-zA-Z]{3,} [0-9]{1,2}, '[0-9]{2}",

	/* Times */
	"[0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}( )?(pm|PM|am|AM)",
	"[0-9]{1,2}:[0-9]{1,2}( )?(pm|PM|am|AM)",
	"[0-9]{1,2}:[0-9]{1,2}:[0-9]{1,2}",
	"[0-9]{1,2}:[0-9]{1,2}",
}
