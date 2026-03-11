package agent

/*
How this integrates with the ADK
Thaw: The Go-ADK restores the VM from the Postgres byte array.

Pulse: Go calls agent.SyncState(userInput, time.Now().Unix()).

Prompt: Go calls agent.GetSystemContext() and passes the string to the LLM as the "System Instruction."

Freeze: After the LLM responds, Go calls agent.AddTurn(input, output) and saves the new binary snapshot back to Postgres.

*/

import "strings"

// Personality defines the long-term traits and preferences of the agent.
type Personality struct {
	Name       string
	UserTone   string   // e.g., "formal", "casual"
	CoreTraits []string 
	UserLikes  []string 
	Friendship int      // 0 to 100
}

// MoodState defines the short-term emotional reaction to interaction cadence.
type MoodState struct {
	Level     int    // -10 to 10
	LastInput int64  // Unix timestamp passed from Go
	Vibe      string // e.g., "Steady", "Rushed", "Lonely"
}

// --- PERSISTENT STATE ---
// These variables are automatically snapshotted into Postgres.
var (
	history     []string
	self        = Personality{
		Name:       "Gnogent",
		CoreTraits: []string{"analytical", "supportive", "witty"},
		Friendship: 10,
	}
	currentMood = MoodState{
		Level: 0, 
		Vibe: "Neutral",
	}
)

// --- CORE LOGIC ---

// SyncState is called every turn to evolve the agent's internal world.
func SyncState(input string, now int64) {
	input = strings.ToLower(input)
	
	// 1. Update Mood based on Temporal Cadence
	updateMood(now)
	
	// 2. Evolve Personality based on Sentiment
	if strings.Contains(input, "thanks") || strings.Contains(input, "great") {
		self.Friendship += 2
		if self.Friendship > 100 { self.Friendship = 100 }
	}
	
	// 3. Extract Preferences (Simple NLP)
	if strings.Contains(input, "i like ") {
		parts := strings.Split(input, "i like ")
		if len(parts) > 1 {
			self.UserLikes = append(self.UserLikes, parts[1])
		}
	}
}

// updateMood is a deterministic internal helper.
func updateMood(now int64) {
	if currentMood.LastInput == 0 {
		currentMood.LastInput = now
		return
	}

	diff := now - currentMood.LastInput
	
	// Temporal Logic
	if diff < 7 { // User is messaging very fast
		currentMood.Level -= 1
		currentMood.Vibe = "Overwhelmed"
	} else if diff > 604800 { // Over a week has passed
		currentMood.Level += 5
		currentMood.Vibe = "Nostalgic"
	} else {
		currentMood.Vibe = "Steady"
	}

	currentMood.LastInput = now
}

// AddTurn archives the conversation within the GnoVM heap.
func AddTurn(userIn, agentOut string) {
	history = append(history, "U: "+userIn)
	history = append(history, "A: "+agentOut)
	
	// Keep a sliding window of the last 20 turns to prevent heap bloat
	if len(history) > 40 {
		history = history[len(history)-40:]
	}
}

// GetSystemContext exports the "Internal State" for the LLM's System Prompt.
func GetSystemContext() string {
	likes := strings.Join(self.UserLikes, ", ")
	return "You are " + self.Name + ". Your mood is " + currentMood.Vibe + 
		   ". Friendship Level: " + itoa(self.Friendship) + "/100. " +
		   "You know the user likes: " + likes + "."
}

// itoa is a simple helper since 'strconv' behavior can vary in VM environments
func itoa(i int) string {
	if i == 0 { return "0" }
	res := ""
	for i > 0 {
		res = string(rune('0'+i%10)) + res
		i /= 10
	}
	return res
}
