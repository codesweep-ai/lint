package prose

import "strings"

func splitFields(s string) []string { return strings.Fields(s) }

// sharedVerbs is what a sentence needs one of to be a sentence.
//
// Deliberately generous: the check is for epigrams with no verb at all, not
// for unusual verbs. This is the union of what every project using this linter
// has needed. A project adds only what is its own, through projectVerbs in the
// configuration file; an ordinary English verb belongs here, where every
// project gets it.
//
// The last group is the past tense, and it was the last to arrive. A document
// says what the software is, so the list grew up in the present; a ledger
// record says what happened, and the first corpus of records read reported
// "A third instance sat in an example commit message." as an epigram with no
// verb.
const sharedVerbs = `
is|are|was|were|be|been|being|has|have|had|does|do|did|can|cannot|could|may|
might|must|should|would|will|serves?|holds?|keeps?|makes?|takes?|gives?|
gets?|goes|comes?|runs?|sends?|reads?|writes?|records?|replays?|matches|
means?|needs?|names?|shows?|says?|calls?|called|tells?|lets?|leaves?|puts?|
adds?|drops?|
refuses?|reports?|carries|carry|costs?|works?|fails?|exists?|belongs?|
applies|apply|cover(?:s|ed)?|happens?|arrives?|appears?|starts?|stops?|waits?|wants?|uses?|
sits?|lives?|turns?|appends?|aligns?|differs?|blanks?|restores?|proposes?|
checks?|answers?|asks?|ships?|gates?|grab|see|verify|compares?|streams?|
beats?|part|updates?|prints?|install|build|produces?|requires?|expects?|
prefers?|avoids?|treats?|points?|binds?|forwards?|spends?|contacts?|
reach(?:es)?|decides?|chooses?|splits?|joins?|stores?|loads?|opens?|closes?|
follows?|describes?|explains?|lists?|pins?|hooks?|collects?|removes?|
deletes?|creates?|allows?|blocks?|passes?|picks?|sorts?|contains?|includes?|
involves?|supports?|offers?|returns?|accepts?|ignores?|skips?|ends?|begins?|
spans?|tracks?|marks?|flags?|counts?|emits?|wraps?|wins?|fights?|helps?|
meets?|breaks?|hides?|lands?|leads?|routes?|builds?|assembles?|behaves?|
settles?|strips?|throws?|hangs?|dials?|exposes?|guards?|changes?|proxies|
proxy|consults?|negotiates?|matched|matching|selects?|introduces?|walks?|
reaches?|stays?|target|resolves?|state|say|note|fold|split|number|cut|
delete|links?|output|discovers?|produced|caught|moves?|repoint|fix|becomes|
measure|generate|capture|paste|feed|check|run|add|show|list|name|bury|
documents?|confirm|record|report|prove|extend|tune|wire|ship|drop|keep|sets?|
bumps?|catches|catch|edits?|copies|copy|write|read|pass|tr(?:y|ies|ied)|editing|
pointing|running|recording|replaying|calling|adding|search|classify|reorder|
imports?|invalidates?|renders?|validates?|iterates?|quote|reproduce|rewrite|matters?|
welcomes?|fills?|sizes?|declares?|computes?|maps?|opt|exceeds?|derives?|receives?|
mint|land|share|scrolls?|handles?|assert|drives?|exercise|push|comments?|
refreshes?|scaffolds?|promotes?|named|reuse|render|refresh|validate|mints?|
accretes?|files?|renumbers?|schedules?|scheduled|claims?|claimed|satisfies|
satisfy|self-hosts?|predates?|conforms?|derives?|embeds?|inlines?|surfaces?|
travels?|iterate|pushes?|shares?|handle|teleports?|prompts?|reflows?|occupy|
occupies|said|saw|attribute|downgrade|silencing|clobbered|earns?|accreting|
honest|tunes?|restrict|narrows?|address|reclaim|boots?|reaches|inherits?|
pivots?|disturb|destroys?|recreat(?:e|ing)|mounts?|pulls?|denies?|deny|lends?|
lend|trust|skip|spin|hand|oversee|wall(?:ed)?|specify|specifies|falls?|
addresses|source|chafe|climbs?|adopts?|sweeps?|probes?|volunteers?|merges?|
finds?|hands?|stalls?|head|provide|resolve|exhaust|re-enables?|touch(?:es)?|
anchors?|wedge|trips?|owes?|rules?|shaped?|simplifying|reintroduces?|
rebuild|scrape|cycles?|supersedes?|steps?|enforces?|bounds?|generates?|
suffices?|suffice|
establish(?:es)?|remain(?:s)?|reflects?|performs?|composes?|states?|become|
prevent(?:s)?|talks?|talk|supervis(?:e|es|ing)|delegates?|judges?|
attempts?|attempted|diagnos(?:e|es|ed)|
moved|stayed|sat|printed|held|ran|wrote|showed|took|gave|kept|made|got|went|
came|sent|told|meant|left|failed|passed|returned|reported|opened|closed|
added|dropped|landed|needed|wanted|shipped|began|broke|chose|knew|found`
