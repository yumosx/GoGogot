package telegram

const (
	channelPrefix = "tg_"
	maxMessageLen = 4000

	maxTextFileSize    = 512 * 1024
	maxImageFileSize   = 10 * 1024 * 1024
	maxGenericFileSize = 20 * 1024 * 1024
	maxArchiveEntries  = 20
)

var toolLabel = map[string]string{
	"bash":            "Running command",
	"edit_file":       "Editing file",
	"read_file":       "Reading file",
	"write_file":      "Writing file",
	"list_files":      "Listing files",
	"web_search":      "Searching the web",
	"web_fetch":       "Reading webpage",
	"web_request":     "Making request",
	"web_download":    "Downloading",
	"send_file":       "Sending file",
	"task_plan":       "Planning",
	"memory_read":     "Checking memory",
	"memory_write":    "Saving to memory",
	"memory_list":     "Listing memories",
	"recall":          "Recalling history",
	"schedule_add":    "Scheduling task",
	"schedule_list":   "Listing schedule",
	"schedule_remove": "Removing schedule",
	"soul_read":       "Reading identity",
	"soul_write":      "Updating identity",
	"user_read":       "Reading user profile",
	"user_write":      "Updating user profile",
	"system_info":     "Checking system",
	"skill_read":      "Reading skill",
	"skill_list":      "Listing skills",
	"skill_create":    "Creating skill",
	"skill_update":    "Updating skill",
	"skill_delete":    "Deleting skill",
	"report_status":   "Updating status",
	"send_message":    "Sending message",
	"ask_user":        "Asking user",
}
