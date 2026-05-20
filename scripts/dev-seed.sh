#!/usr/bin/env bash
# Seed a Kkullm development database with three pseudo-projects (beehive,
# birds_nest, ant_hill), their agents, and a representative mix of cards
# and comments. Acts as a reset: existing data under these three projects
# is deleted and recreated.
#
# Requires: a running `kkullm serve` against $KKULLM_DB, and the `kkullm`
# binary on PATH (or at ./kkullm).

set -euo pipefail

KKULLM_SERVER="${KKULLM_SERVER:-http://localhost:7722}"
KKULLM_DB="${KKULLM_DB:-kkullm.db}"

SEED_PROJECTS=(beehive birds_nest ant_hill)

KKULLM_BIN="${KKULLM_BIN:-kkullm}"
if ! command -v "$KKULLM_BIN" >/dev/null 2>&1; then
  if [[ -x ./kkullm ]]; then
    KKULLM_BIN=./kkullm
  else
    echo "error: 'kkullm' binary not found on PATH and ./kkullm is not executable" >&2
    echo "       Run 'task build' first." >&2
    exit 1
  fi
fi

# --- helpers ----------------------------------------------------------------

confirm() {
  local prompt=$1 answer
  read -r -p "$prompt (type 'yes' to continue): " answer
  if [[ "$answer" != "yes" ]]; then
    echo "Aborted." >&2
    exit 1
  fi
}

check_server() {
  if ! curl -fs -o /dev/null "$KKULLM_SERVER/api/projects"; then
    echo "error: cannot reach Kkullm server at $KKULLM_SERVER" >&2
    echo "       Start it first with: kkullm serve --db $KKULLM_DB" >&2
    exit 1
  fi
}

sqlite_q() {
  sqlite3 "$KKULLM_DB" "$@"
}

# Returns count of cards under the given project name, or 0 if project missing.
count_cards_for() {
  local project=$1
  sqlite_q "SELECT COALESCE((SELECT COUNT(*) FROM cards c JOIN projects p ON c.project_id = p.id WHERE p.name = '$project'), 0);"
}

count_agents_for() {
  local project=$1
  sqlite_q "SELECT COALESCE((SELECT COUNT(*) FROM agents a JOIN projects p ON a.project_id = p.id WHERE p.name = '$project'), 0);"
}

project_exists() {
  local project=$1
  local n
  n=$(sqlite_q "SELECT COUNT(*) FROM projects WHERE name = '$project';")
  [[ "$n" -gt 0 ]]
}

cleanup() {
  # Build the IN-list once; SQLite is happy with the literal quoting.
  local in_list
  in_list=$(printf "'%s'," "${SEED_PROJECTS[@]}")
  in_list="${in_list%,}"

  sqlite_q <<SQL
PRAGMA foreign_keys = ON;
BEGIN;
DELETE FROM cards
 WHERE project_id IN (SELECT id FROM projects WHERE name IN ($in_list));
DELETE FROM agents
 WHERE project_id IN (SELECT id FROM projects WHERE name IN ($in_list));
DELETE FROM projects WHERE name IN ($in_list);
COMMIT;
SQL
}

# --- CLI wrappers -----------------------------------------------------------

kk() {
  "$KKULLM_BIN" --server "$KKULLM_SERVER" "$@"
}

create_project() {
  local name=$1 desc=$2
  kk project create --name "$name" --description "$desc" >/dev/null
}

create_agent() {
  local name=$1 project=$2 bio=$3
  kk agent create --name "$name" --project "$project" --bio "$bio" >/dev/null
}

# create_card <project> <as_agent> <title> <status> [extra flags...]
# Echoes the new card ID to stdout.
create_card() {
  local project=$1 as_agent=$2 title=$3 status=$4
  shift 4
  local output
  output=$(kk --as "$as_agent" --project "$project" card create \
    --title "$title" --status "$status" "$@")
  local id
  id=$(echo "$output" | sed -n 's/^Created card #\([0-9][0-9]*\):.*/\1/p')
  if [[ -z "$id" ]]; then
    echo "error: could not parse card id from output: $output" >&2
    exit 1
  fi
  echo "$id"
}

add_comment() {
  local card_id=$1 as_agent=$2 body=$3
  kk --as "$as_agent" comment create "$card_id" --body "$body" >/dev/null
}

# --- main flow --------------------------------------------------------------

echo "About to seed development data."
echo "  Server: $KKULLM_SERVER"
echo "  DB:     $KKULLM_DB"
echo "  Projects to (re)create: ${SEED_PROJECTS[*]}"
echo
confirm "Continue?"

check_server

declare -a existing=()
for p in "${SEED_PROJECTS[@]}"; do
  if project_exists "$p"; then
    existing+=("$p: $(count_cards_for "$p") cards, $(count_agents_for "$p") agents")
  fi
done

if (( ${#existing[@]} > 0 )); then
  echo
  echo "Existing seed data detected:"
  for line in "${existing[@]}"; do
    echo "  $line"
  done
  echo
  echo "These projects and ALL their cards, comments, and agents will be"
  echo "DELETED and recreated."
  confirm "Proceed with destructive reset?"
  cleanup
  echo "Cleanup complete."
fi

echo "Creating projects..."
create_project beehive    "A buzzing colony with strong opinions about pollen logistics."
create_project birds_nest "A single overworked robin running a one-bird startup."
create_project ant_hill   "Subterranean megacorp with aphid futures and crumb arbitrage."

echo "Creating agents..."
create_agent worker_bee  beehive    "Reliable middle manager of the foraging shift. Knows every clover within 2 miles."
create_agent drone       beehive    "Lives for the mating flight. Mostly just hangs around looking confident."
create_agent queen_bee   beehive    "Sole reproductive authority. Believes delegation is for lesser hives."
create_agent robin       birds_nest "Sole agent, sole parent, sole worm-fetcher. Caffeinated by sheer panic."
create_agent worker_ant  ant_hill   "Carries 50x body weight without complaint. Mostly."
create_agent soldier_ant ant_hill   "Mandibles first, questions later. Strong opinions on wasps."
create_agent queen_ant   ant_hill   "Lays 1,200 eggs/day and still has time for governance."

echo "Creating beehive cards..."

# Considering
BH_RELOCATE=$(create_card beehive worker_bee \
  "Relocate hive entrance to face southeast?" considering \
  --body "Morning sun would warm the comb earlier, but the magnolia branch is in the way and the resident squirrel has Strong Opinions." \
  --tag architecture --tag long-term)

BH_RATIONING=$(create_card beehive queen_bee \
  "Authorize Q3 royal jelly rationing review" considering \
  --assignee queen_bee \
  --body "The drones have been... robust. Reviewing per-larva allocations." \
  --tag governance)

BH_RIVAL=$(create_card beehive worker_bee \
  "Investigate suspicious humming from rival hive" considering \
  --assignee worker_bee \
  --body "Pitch sounds about a semitone flat. Could be reconnaissance. Could be just a bad day." \
  --tag intel)

# Todo
BH_CRACKED=$(create_card beehive worker_bee \
  "Replace cracked honeycomb cell #408" todo \
  --body "Cell #408 has been weeping nectar for three days. Structural concerns." \
  --tag maintenance)

BH_FORAGE=$(create_card beehive worker_bee \
  "Forage *south clover* field at first light" todo \
  --assignee worker_bee \
  --body "Scouts report peak nectar yield 0530-0700. Bring backup proboscis." \
  --tag foraging --tag priority)

BH_MATINGREH=$(create_card beehive queen_bee \
  "Attend mating flight rehearsal (mandatory)" todo \
  --assignee drone \
  --body "Yes, again. Yes, all of you. No, 'I have plans' is not a valid excuse." \
  --tag training)

BH_SIGN=$(create_card beehive worker_bee \
  "Repaint hive sign — 'Bee Right Back'" todo \
  --assignee drone --assignee worker_bee \
  --body "Current sign reads 'Bee Right Bac' after the storm. Aesthetics matter." \
  --tag pr)

BH_MARKDOWN_SHOWCASE_BODY=$'## Subhead\n\nA paragraph with **bold**, _italic_, ~~strikethrough~~, `inline code`, and an autolink: https://example.com.\n\n### Lists\n\n- bullet one\n- bullet two\n  - nested\n- bullet three\n\n1. ordered\n2. ordered\n3. ordered\n\n### Task list\n\n- [x] design approved\n- [ ] implementation\n- [ ] tests\n\n### Code\n\n```go\nfunc render(md string) template.HTML {\n    return mdRenderer.Render(md)\n}\n```\n\n### Table\n\n| Status     | Count |\n|------------|------:|\n| todo       |     3 |\n| in_flight  |     1 |\n| completed  |     7 |\n\n> A blockquote for emphasis.\n\n![Diagram](https://placehold.co/600x200)\n\n[Link to docs](https://example.com/docs)\n'

BH_SHOWCASE=$(create_card beehive worker_bee \
  "Markdown ~~smoke~~ **showcase** with \`code\`" todo \
  --body "$BH_MARKDOWN_SHOWCASE_BODY" \
  --tag docs)

BH_SHOWCASE_C1=$'Here is what I have in mind:\n\n```bash\ntask build && ./kkullm serve --db kkullm.db\n```\n\n- step 1\n- step 2\n- step 3\n'
add_comment "$BH_SHOWCASE" drone "$BH_SHOWCASE_C1"

BH_SHOWCASE_C2=$'Looks good. Linking the [issue](https://example.com/issues/42) and emphasizing **the deadline**.'
add_comment "$BH_SHOWCASE" queen_bee "$BH_SHOWCASE_C2"

# In flight
BH_POLLEN=$(create_card beehive worker_bee \
  "Process pollen haul from yesterday's foray" in_flight \
  --assignee worker_bee \
  --body "Pack, ferment, store. Standard procedure, but the volume is unusually high." \
  --tag production \
  --blocked-by "$BH_FORAGE")

BH_WALKABOUT=$(create_card beehive queen_bee \
  "Royal walkabout: inspect brood chamber B" in_flight \
  --assignee queen_bee \
  --body "Personal inspection of new brood. Workers, please clear the corridor." \
  --tag governance)

BH_TRUCE=$(create_card beehive queen_bee \
  "Negotiate truce with bumble cooperative" in_flight \
  --assignee worker_bee --assignee drone \
  --body "Territory disputes over the lavender bed. Bumbles claim historical use." \
  --tag diplomacy)

BH_AERIAL=$(create_card beehive queen_bee \
  "Aerial survey of new flower patch beyond the fence" in_flight \
  --assignee drone --assignee robin \
  --body "Robin has agreed to fly recon — higher altitude, better vantage. We owe her one." \
  --tag intel --tag cross-project)

# Blocked
BH_MATING=$(create_card beehive queen_bee \
  "Schedule actual mating flight" blocked \
  --assignee drone \
  --body "Cannot proceed until rehearsal attendance is confirmed." \
  --tag governance \
  --blocked-by "$BH_MATINGREH")

BH_SUCCESSOR=$(create_card beehive queen_bee \
  "Successor selection committee" blocked \
  --assignee queen_bee \
  --body "Strictly contingency. The Queen is fine. The Queen is FINE." \
  --tag governance --tag sensitive \
  --blocked-by "$BH_WALKABOUT")

# Completed
BH_WAX=$(create_card beehive worker_bee \
  "Wax production target for May" completed \
  --assignee worker_bee \
  --body "Hit 112% of target. Surplus stored in chamber D." \
  --tag production)

BH_SWARM=$(create_card beehive queen_bee \
  "Annual swarm season risk assessment" completed \
  --assignee queen_bee --assignee worker_bee \
  --body "Low risk this year. Drone population stable. Queen mood: serene." \
  --tag governance)

BH_FAILEDFLIGHT=$(create_card beehive drone \
  "Failed mating flight — postmortem" completed \
  --assignee drone \
  --body "Wind shear at 12m. Three drones returned. Two did not. We do not speak of this." \
  --tag retrospective)

# Tabled
BH_MAT=$(create_card beehive worker_bee \
  "Proposal: install tiny welcome mat at entrance" tabled \
  --assignee worker_bee \
  --body "Aesthetic upgrade. Cost-benefit unclear. Revisit in autumn." \
  --tag aesthetics)

echo "Creating birds_nest cards..."

# Considering
BN_TWIGS=$(create_card birds_nest robin \
  "Reinforce nest with stronger twigs after storm forecast" considering \
  --body "Weather radar shows a front Thursday. Current twig grade: questionable." \
  --tag structural)

BN_5EGG=$(create_card birds_nest robin \
  "Should we go for a 5th egg this season?" considering \
  --assignee robin \
  --body "Pro: more chicks. Con: I am one (1) bird." \
  --tag long-term)

BN_LEFT=$(create_card birds_nest robin \
  "Consider moving nest 6 inches left (sun angle)" considering \
  --assignee robin \
  --body "Would help with afternoon shade. Also: I have already built this nest." \
  --tag architecture)

# Todo
BN_PATROL=$(create_card birds_nest robin \
  "Worm patrol: 5am shift" todo \
  --body "Standing item. Always. Forever. Until the chicks fledge or I do." \
  --tag foraging --tag recurring)

BN_SQUEAKY=$(create_card birds_nest robin \
  "Brood chick #2 — squeakiest one, needs extra feeding" todo \
  --assignee robin \
  --body $'Squeaks at all hours. Suspect inherited it from his father (RIP). Plan:\n\n- [x] inventory worms\n- [ ] extra feeding at dawn\n- [ ] extra feeding at dusk' \
  --tag brood)

BN_SQUIRREL=$(create_card birds_nest robin \
  "Repel that one squirrel that keeps Looking" todo \
  --assignee robin \
  --body "He just sits on the branch. Looking. He never DOES anything. But he LOOKS." \
  --tag security)

# In flight
BN_WARMING=$(create_card birds_nest robin \
  "Continuous egg-warming (literally cannot stop)" in_flight \
  --assignee robin \
  --body "Status: sitting. Have been sitting. Will continue sitting. Send snacks." \
  --tag brood --tag priority)

BN_LAUNCH=$(create_card birds_nest robin \
  "Fledgling launch coaching: chick #1" in_flight \
  --assignee robin \
  --body "He's almost ready. He says he's ready. He is not ready." \
  --tag brood)

BN_FLIGHTPATHS=$(create_card birds_nest robin \
  "Catalog beehive flight paths for collision avoidance" in_flight \
  --assignee robin --assignee worker_bee \
  --body "Joint initiative with the hive. Last week's near-miss was educational for everyone." \
  --tag intel --tag cross-project)

# Blocked
BN_BREAK=$(create_card birds_nest robin \
  "Take a break, eat something, drink water" blocked \
  --assignee robin \
  --body "Cannot proceed while egg-warming is in flight. There is no relief shift. There is only Robin." \
  --tag wellbeing \
  --blocked-by "$BN_WARMING")

BN_PARTNER=$(create_card birds_nest robin \
  "Find life partner replacement (the last one left)" blocked \
  --assignee robin \
  --body "Cannot interview candidates while continuously immobilized on nest. Catch-22." \
  --tag sensitive --tag long-term \
  --blocked-by "$BN_BREAK")

# Completed
BN_BUILD=$(create_card birds_nest robin \
  "Built nest in fork of tree" completed \
  --assignee robin \
  --body "Three days, ~400 twigs, one (1) discarded shoelace for structural integrity." \
  --tag structural)

BN_LAID=$(create_card birds_nest robin \
  "Laid four eggs" completed \
  --assignee robin \
  --body "All four viable. No notes." \
  --tag brood)

BN_WIND=$(create_card birds_nest robin \
  "Defeated wind event of April 3rd" completed \
  --assignee robin \
  --body "Nest held. Robin held. Shoelace held. Everyone is a hero." \
  --tag retrospective)

# Todo cont. — cross-project complaint
BN_COMPLAINT=$(create_card birds_nest robin \
  "File formal complaint about ant_hill expansion under tree" todo \
  --assignee robin \
  --body "They are tunneling. I can feel the tunneling. The roots are concerned." \
  --tag diplomacy --tag cross-project)

# Tabled
BN_SABBATICAL=$(create_card birds_nest robin \
  "Sabbatical in South America" tabled \
  --assignee robin \
  --body "Hilarious. Comedy gold. Filed under 'maybe next decade'." \
  --tag aspirational)

echo "Creating ant_hill cards..."

# Considering
AH_ANNEX=$(create_card ant_hill worker_ant \
  "Annex south meadow — feasibility study" considering \
  --body "Untapped aphid territory. Also: contested airspace with hive. Politically delicate." \
  --tag expansion --tag long-term)

AH_POPGROWTH=$(create_card ant_hill queen_ant \
  "Population growth strategy Q3" considering \
  --assignee queen_ant \
  --body "Current output sustainable but unambitious. Considering doubling brood chamber capacity." \
  --tag governance)

AH_PERIMETER=$(create_card ant_hill soldier_ant \
  "Build defensive perimeter near robin tree" considering \
  --assignee soldier_ant \
  --body "The bird is large and territorial. Recommend non-aggression posture." \
  --tag security)

# Todo
AH_DRAIN=$(create_card ant_hill worker_ant \
  "Dig drainage tunnel before rainy season" todo \
  --body "Last year's flood took out Tunnel C. We do not repeat that." \
  --tag infrastructure --tag priority)

AH_CRUMBS=$(create_card ant_hill worker_ant \
  "Transport breadcrumb shipment from picnic area" todo \
  --assignee worker_ant \
  --body $'Crumb yield has dipped. Running:\n\n```sh\nfind /pantry -name "*.crumb" -newer last_haul\n```\n\nResults pending. Pretzel chunks especially valuable.' \
  --tag logistics)

AH_APHIDQUOTA=$(create_card ant_hill queen_ant \
  "Aphid \`milking-quotas\` review" todo \
  --assignee worker_ant \
  --body "Honeydew yield per aphid is down 12%. Investigating cause: stress? rival hive interference?" \
  --tag production)

AH_PATROL=$(create_card ant_hill soldier_ant \
  "Patrol north flank against rival colony" todo \
  --assignee soldier_ant \
  --body "Reds. Always the reds. Standard protocol." \
  --tag security --tag recurring)

# In flight
AH_LEAF=$(create_card ant_hill worker_ant \
  "Process leaf cuttings into fungus garden" in_flight \
  --assignee worker_ant \
  --body "Chamber F is at 70% capacity. Mycelium growth ahead of schedule." \
  --tag production)

AH_BUMBLES=$(create_card ant_hill soldier_ant \
  "Repel bumblebee delegation (again)" in_flight \
  --assignee soldier_ant --assignee worker_ant \
  --body "They keep landing on OUR aphids. Diplomatic channels exhausted." \
  --tag diplomacy --tag cross-project)

AH_LAY=$(create_card ant_hill queen_ant \
  "Lay eggs (1,200/day target)" in_flight \
  --assignee queen_ant \
  --body "On pace. Slight uptick after switching to the new royal jelly supplier." \
  --tag brood --tag recurring)

AH_TUNNELEXT=$(create_card ant_hill worker_ant \
  "Tunnel network east extension, phase 2" in_flight \
  --assignee worker_ant \
  --body "Extending corridor toward the rose bushes. Aphid real estate." \
  --tag infrastructure)

# Blocked
AH_APHIDMOVE=$(create_card ant_hill worker_ant \
  "Aphid colony relocation to east tunnels" blocked \
  --assignee worker_ant \
  --body "Cannot move livestock until destination tunnels reach the bushes." \
  --tag logistics \
  --blocked-by "$AH_TUNNELEXT")

AH_HUMMING=$(create_card ant_hill soldier_ant \
  "Investigate suspicious humming above" blocked \
  --assignee soldier_ant \
  --body "Vibration from above ground. Could be threat. Could be bees. (It is bees.)" \
  --tag intel --tag cross-project \
  --blocked-by "$AH_PATROL")

# Completed
AH_WONDERBREAD=$(create_card ant_hill worker_ant \
  "Operation Wonderbread — successful crumb extraction" completed \
  --assignee worker_ant --assignee soldier_ant \
  --body "47 ants. 1 picnic. Zero casualties. The mustard packet remains contested." \
  --tag logistics --tag retrospective)

AH_SUCCESSION=$(create_card ant_hill queen_ant \
  "Quarterly succession plan filed" completed \
  --assignee queen_ant \
  --body "Updated heir designations. Sealed. Witnessed. Filed under chamber Q." \
  --tag governance)

AH_APHIDREPORT=$(create_card ant_hill worker_ant \
  "Aphid yield report — April" completed \
  --assignee worker_ant \
  --body "Honeydew yield: 340L equivalent. Slight dip late month (see quota review)." \
  --tag production)

# Tabled
AH_WASPS=$(create_card ant_hill soldier_ant \
  "Negotiate non-aggression pact with wasps" tabled \
  --assignee soldier_ant \
  --body "Wasps non-responsive. Wasps unpredictable. Wasps. Tabled indefinitely." \
  --tag diplomacy)

echo "Adding comments..."

# beehive — multi-agent
add_comment "$BH_FORAGE" drone      "I'll join. Need the airtime hours anyway."
add_comment "$BH_FORAGE" worker_bee "Wheels up 0515 sharp. Bring the small baskets, not the parade ones."

add_comment "$BH_POLLEN" queen_bee  "Prioritize the dark amber lot — that's brood-grade."
add_comment "$BH_POLLEN" worker_bee "Already segregated. Chamber B, third row."

add_comment "$BH_SUCCESSOR" queen_bee "Discretion above all. We name no names yet."
add_comment "$BH_SUCCESSOR" drone     "I would like to formally not be considered."

# beehive cross-project comments
add_comment "$BH_AERIAL"  robin    "I'll do a high pass at dawn. If I see anything yellow that isn't a flower, you'll hear about it."
add_comment "$BH_AERIAL"  drone    "Appreciated. We'll have the cell open in the comb for your invoice."
add_comment "$BH_SWARM"   queen_ant "Royal solidarity. May your swarm season be uneventful and your aphids unmolested."

# birds_nest — multi-agent / cross-project
add_comment "$BN_FLIGHTPATHS" worker_bee "Mapped the morning corridor. Suggest robin stays above 4m between 0600-0900."
add_comment "$BN_FLIGHTPATHS" drone      "Seconded. Some of us are not fast learners about altitude."
add_comment "$BN_FLIGHTPATHS" robin      "Acknowledged. I will also try not to eat anyone."

add_comment "$BN_BREAK"   queen_bee  "Robin. DELEGATE. (We know. We know there's no one. But still.)"
add_comment "$BN_BREAK"   worker_ant "If it helps, we found half a granola bar near the picnic area. It's yours."
add_comment "$BN_BREAK"   robin      "I appreciate you. I cannot move."

add_comment "$BN_SQUEAKY" queen_bee  "Have you tried giving him a job? Works for drones."
add_comment "$BN_SQUEAKY" robin      "He IS 11 days old."

add_comment "$BN_COMPLAINT" worker_ant "We are tunneling AROUND the roots, with respect. Documentation forthcoming."
add_comment "$BN_COMPLAINT" queen_ant  "Sister-of-the-Air, let us schedule a summit. Bring grievances; we'll bring crumbs."
add_comment "$BN_COMPLAINT" robin      "Fine. Tuesday. The branch over the bird bath. Bring crumbs."

# ant_hill — multi-agent
add_comment "$AH_CRUMBS" queen_ant  "Pretzel salt is non-negotiable. Bring the salt."
add_comment "$AH_CRUMBS" worker_ant "Salt secured. Mustard packet is en route under separate escort."

add_comment "$AH_BUMBLES" worker_ant "They keep apologizing and then doing it again."
add_comment "$AH_BUMBLES" soldier_ant "Apologies are not aphid milk."
add_comment "$AH_BUMBLES" drone       "Hi yes — speaking on behalf of the hive — that's not us, those are bumbles, separate organization, totally different vibe, please don't bite us."

# ant_hill — cross-project comments
add_comment "$AH_HUMMING"    queen_bee "That is, in fact, us. Apologies. We are recalibrating choir practice."
add_comment "$AH_HUMMING"    soldier_ant "Acknowledged. Standing down. For now."
add_comment "$AH_SUCCESSION" queen_bee  "Filed and witnessed on our end as well. Continuity is everything."
add_comment "$AH_ANNEX"      robin      "From altitude: the south meadow is wetter than it looks. Plan for drainage."

echo
echo "Seed complete."
echo "  beehive:    18 cards across worker_bee, drone, queen_bee"
echo "  birds_nest: 16 cards (robin is doing her best)"
echo "  ant_hill:   17 cards across worker_ant, soldier_ant, queen_ant"
