# Kkullm Product Sketching

What is Kkullm?

## Thoughts

- Kkullm is a new agent orchestration system
- Uses the classic blackboard pattern
- Similarities to kanban and slack-like team chat
- enables messaging between user and agents, and between agents
- has a simple CLI to make interaction extremely efficient
- has a web UI for human user
- has notifications for user
- Claude Code hook integration pulls actionable cards on startup
- Agent can update their own cards, request help from the user, and request help from other agents
- Agents also have a profile within Kkullm to help with understanding
  - responsibilities/roles
  - capabilities
- Kkullm has different kinds of cards
  - tasks: prompts to do work
  - RFCs: requests for comments, generally open or targeted at specific agents

### System Schema

When participating agents launch, they must disambiguate between an unattended session and a
user-interactive session.  For an unattended session, the agent
- (by hook) pulls the list of actionable cards;
- selects the highest priority card,
- and composes a prompt, outputting or saving it before it terminates.
- the re-launched agent acts on the composed prompt.

This allows for prioritization as a discrete step, and prioritization is done in the context of all
outstanding actionable cards.  This way, duplicated requests can be taken into account, as well as
tasks which are interdependent.  The composed prompt shoudl reference any such dependencies or duplicates,
but should omit any other information which is not relevant to the task at hand.

The agent then works on the prompt, and updates any cards which are affected by the work.  This does not
mean completing all referenced cards!  It means completing the one top priority card, but then also updating
any other cards which are affected by the work done on the top priority card.

For user-interactive sessions, the agent and user can work together to compose the prompt, and then the user
may restart, or `/clear` or `/compact` and then load the prompt and commence work collaboratively.

On the board, an agent may request a collaborative session with the user, or may request clarification on
a card itself.

### Card Schema

A card has these properties:

- **title**: a short, concise, and informative title (two lines max)
- **body**: a longer, more detailed description of the card (unlimited length)
- **assignee**: the entities responsible for the card, whether human or agent
- **comments**: the conversation stream between agent(s) and user
- **project**: the project where the work on the card is to be done
- **tags**: a list of tags, which can be used to group cards
- **related**: a list of relationships to other cards, including:
  - blocked_by: this card is blocked by the other card
  - belongs_to: this card is a sub-task of the other card
  - interested_in: the other card is of interest to this card
- **status**: the status of the card, which can be one of:
  - considering: the card is being considered (read, comment/answer, but do not work on it yet)
  - todo: the card is ready for to be pulled by an agent (끌림 - Kkullm!)
  - in_flight: the card is being worked on
  - completed: the card is complete
  - done: the card is closed, and no further work is expected
  - tabled: the card is closed but not completed, and may be reopened by the user
  - blocked: the card is blocked, and no further work is expected
