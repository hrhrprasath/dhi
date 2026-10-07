package prompt

import (
	"fmt"
	"strings"
)

const SystemOneInstructions = "Route this user input to exactly one option: the single pipeline whose purpose best matches it. " +
	"Default to CONVERSATION for anything that is not a clear actionable request. " +
	"A URL anywhere in the input means EXECUTE_FETCH_URL. " +
	"Reading or summarizing a page means EXECUTE_FETCH_URL; searching within a website means EXECUTE_SITE_SEARCH. " +
	"'remind me to <task>' is a reminder, so EXECUTE_TODO; 'schedule <event>' or repeating work is EXECUTE_SCHEDULE."

const systemOneConversation = "Chat, greetings, questions, opinions, explanations, creative content, small talk, " +
	"or when no pipeline matches"

func BuildSystemOneCriteria(pipelines []PipelineInfo) map[string]string {
	criteria := make(map[string]string, len(pipelines)+1)
	for _, p := range pipelines {
		desc := p.Description
		if len(p.Keywords) > 0 {
			desc += " | triggers: " + strings.Join(p.Keywords, ", ")
		}
		criteria[fmt.Sprintf("EXECUTE_%s", strings.ToUpper(p.Name))] = desc
	}
	criteria["CONVERSATION"] = systemOneConversation
	return criteria
}
