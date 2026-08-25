package prose

// Source names the style guide a rule takes its lead from.
//
// The house rules came out of reviewing what actually confuses readers of
// these projects. Where a rule matches published guidance, it says so, because
// a writer arguing with a rule deserves to know whether it is one project's
// preference or an industry convention with a page behind it.
type Source string

const (
	// House is this project's own, from repeated review of its documents.
	House Source = ""
	// Google is the Google developer documentation style guide,
	// https://developers.google.com/style.
	Google Source = "Google"
	// RedHat is the Red Hat supplementary style guide,
	// https://redhat-documentation.github.io/supplementary-style-guide/.
	RedHat Source = "Red Hat"
	// Both is where the two guides agree.
	Both Source = "Google, Red Hat"
	// Machine is the register that gives away text a model wrote, catalogued
	// by Wikipedia's WikiProject AI Cleanup at
	// https://en.wikipedia.org/wiki/Wikipedia:Signs_of_AI_writing.
	Machine Source = "WikiProject AI Cleanup"
)

// Term is a word the house has decided against, what to write instead, and the
// guidance behind the decision.
//
// Every pattern is matched case-insensitively, so a name that has to be
// checked for its capitals turns the flag off for itself with (?-i:…).
type Term struct {
	Pattern string
	Advice  string
	Source  Source
}

// sharedTerms is the house's own table.
//
// It is deliberately short. A rule earns a place here by catching something a
// writer would fix on being shown it, in a document set this size. A large
// published style sheet applied whole reports thousands of findings, most of
// them that sheet's house disagreeing with yours, and two such sheets
// contradict each other. A rule that mostly reports a preference trains
// everyone to read past the whole linter.
var sharedTerms = []Term{
	// Plain English.
	{`utili[sz]e`, "Use 'use'.", Both},
	{`leverage`, "Use 'use'.", Both},
	{`in order to`, "Use 'to'.", Both},
	{`allows you to`, "Use 'lets you'.", Google},

	// Preferred spellings.
	{`e-mail`, "Use 'email'.", Google},
	{`and/or`, "Write 'and', or 'or'.", Both},

	// Inclusive language.
	{`white-?list`, "Use 'allowlist'.", Both},
	{`black-?list`, "Use 'denylist'.", Both},
	{`master branch`, "Use 'main branch'.", Both},
	{`sanity check`, "Use 'quick check'.", Both},
	{`\bdummy\b`, "Use 'placeholder'.", RedHat},
	{`\bhe or she\b`, "Use 'they'.", Google},

	// Empty intensifiers.
	{`\bsimply\b`, "Delete it: if it were simple the reader would not be reading.", Both},
	{`\beasily\b`, "Delete it.", Both},

	// Names, checked for their capitals.
	{`(?-i:Github|GitHUB)`, "Write 'GitHub'.", Both},
	{`(?-i:Javascript)`, "Write 'JavaScript'.", Both},
	{`(?-i:Mac OS|MacOS)`, "Write 'macOS'.", Both},

	// Latin.
	{`\be\.g\.`, "Write 'for example'.", Both},
	{`\bi\.e\.`, "Write 'that is'.", Both},

	// A word that dates the page.
	{`\b(?:currently|for now|at present|nowadays)\b`,
		"Say what is true, not when: a time-bound word dates the page.", Google},

	// Typography.
	{`\bfrom \d+-\d+`, "Write a range as 'from N to M', or as N-M without 'from'.", Google},
	{`\w+\(s\)`, "Write the plural, or rewrite the sentence.", Google},
	{`\w![ \n]`, "Drop the exclamation mark.", Google},
	{`[.!?]  +[A-Z]`, "One space after a full stop.", Both},

	// Accessibility: a colour naming something the reader has to find. "The
	// tests report green" is a metaphor for passing and is left alone; "the
	// green button" is an instruction nobody can follow in monochrome.
	// The register a model reaches for. Every entry here is a word that says
	// nothing a plainer one does not, and together they are the strongest tell
	// that nobody read the sentence back. Kept short on purpose: a long list of
	// ordinary English would fire on writing that is merely dull.
	{`\bdelves?\b|\bdelving\b`, "Say what it does. 'Delve' is a model's word for 'read'.", Machine},
	{`\btapestry\b|\brich (?:history|heritage|tradition)\b`,
		"Cut it. A metaphor this tired carries no information.", Machine},
	{`\b(?:realm|landscape) of\b`, "Name the thing. 'The landscape of X' is 'X'.", Machine},
	{`\ba testament to\b`, "Say what it shows, and how you know.", Machine},
	{`\bpivotal\b|\bcrucially\b`, "Say why it matters, or drop the adjective.", Machine},
	// Only the verb, and only where a determiner proves it is one. A document
	// that spells the character out, as "letters, digits, dot, dash or
	// underscore" does, is naming it rather than reaching for a longer word.
	{`\bunderscor(?:es|ed|ing)\s+(?:the|that|how|its|their|this|a|an)\b`,
		"Use 'shows' or 'means'.", Machine},
	{`\bseamless(?:ly)?\b`, "Say what happens instead. Nothing is seamless.", Machine},
	{`\bboasts\b`, "Use 'has'.", Machine},
	{`\b(?:cutting-edge|groundbreaking|state-of-the-art)\b`,
		"Cut it. A reader decides that, not the documentation.", Machine},
	{`\bmeticulous(?:ly)?\b`, "Cut it, or say what care was taken.", Machine},
	{`\bintricate\b`, "Use 'complex', or say what makes it so.", Machine},
	{`(?:^|[.!?]\s+)(?:Additionally|Furthermore|Moreover|In conclusion|In summary)\b`,
		"Start with the point. These transitions carry nothing.", Machine},
	{`\bnot (?:just|only) [^,.]{2,40}? but (?:also )?\b`,
		"State what it is. The 'not just X but Y' shape sets up a contrast nobody asked for.", Machine},
	// The editorial sense only: an opener or an aside. "Accreting honestly" is
	// an adverb of manner and says something.
	{`(?:^|[.!?]\s+)(?:Honestly|Frankly)\b|,\s*(?:honestly|frankly),|` +
		`\bto be honest\b|\btruth be told\b`,
		"Delete it. It implies the rest was not.", Machine},
	{`\bit(?:'s| is) (?:important|worth) to note\b|\bit(?:'s| is) important to (?:note|remember)\b`,
		"Delete the frame and keep the fact.", Machine},

	{`\b(?:green|red|amber|blue|yellow)\s+(?:button|icon|light|badge|dot|bar|` +
		`marker|arrow|box|highlight|link|tab|banner)\b`,
		"Name the control as well as its colour: a reader in monochrome has to find it too.",
		House},
}

// sharedTermsProse holds what applies to prose but not to a spec, whose
// register is different. A spec's rationale legitimately says "our own
// address" about the project's own artifacts, so this is checked only where a
// reader is addressed.
var sharedTermsProse = []Term{
	{`\b(?:we|our|ours)\b`, "Address the reader as 'you'; the docs have no 'we'.", Google},
}

// lyAdjectives are adjectives that end in -ly and do take a hyphen in a
// compound. An adverb never does: it already modifies what follows it.
var lyAdjectives = map[string]bool{
	"costly": true, "early": true, "friendly": true, "likely": true,
	"lively": true, "lonely": true, "only": true, "daily": true,
	"weekly": true, "monthly": true, "yearly": true, "deadly": true,
	"silly": true, "ugly": true, "holy": true, "orderly": true,
	"timely": true, "unlikely": true,
}

// throatClearing are frames that comment on the writing instead of getting on
// with it. Each one is a phrase you can delete without losing a fact.
var throatClearing = []string{
	"it is worth", "worth stating", "worth noting", "worth saying", "put simply",
	"simply put", "the point is", "needless to say", "suffice it to say",
	"stated plainly", "to be clear", "in other words", "that said,",
}

// common are words frequent enough that repeating them says nothing about the
// sentence, plus CODE, which is what an inline code span is replaced with.
var common = words(`the a an and or of to in is are was were be it its this that these those
for with on at by as not no so if but who whom whose one two all any each every some
have has had do does did can could may might must should would will code been being
into onto from over under about through within without after before where while both
against
only also other others same such very more most than then there here when what which`)

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range splitFields(s) {
		out[w] = true
	}
	return out
}
