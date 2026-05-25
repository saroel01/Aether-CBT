const fs = require("fs");
const raw = fs.readFileSync("contoh_soal/KIMIA_XII_UAS_2025 (Published)/data/player.js", "utf8");

console.log("=== EXPERT ANALYSIS OF iSPRING PLAYER ===\n");

// 1. Look for the main player class and exposed methods
console.log("1. Searching for main player entry points...");
const playerEntry = raw.match(/QuizPlayer\.(start|prototype|ISPQuizPlayer)/gi);
console.log("QuizPlayer references:", playerEntry ? playerEntry.length : 0);

// 2. Find result submission logic
console.log("\n2. Result submission / webhook logic...");
const submitMatches = raw.match(/\.(submit|sendResult|postResult|sendQuizResult|onQuizComplete)/gi);
console.log("Submission method references:", submitMatches ? submitMatches.length : 0);

// Get context around actual result sending
const sendIdx = raw.search(/sendResult|postResult|quizResults|serverSubmit/i);
if (sendIdx > 0) {
  console.log("\n--- Context around result sending ---");
  console.log(raw.substring(sendIdx - 100, sendIdx + 400));
}

// 3. Authorization bypass potential
console.log("\n3. Authorization handling...");
const authIdx = raw.search(/AuthorizationSlide|authForm|authorizationData|validateAuth/i);
if (authIdx > 0) {
  console.log(raw.substring(authIdx - 50, authIdx + 600));
}

// 4. Programmatic answer setting
console.log("\n4. Answer / interaction model...");
const answerIdx = raw.search(/setAnswer|applyAnswer|userAnswer|recordAnswer/i);
if (answerIdx > 0) {
  console.log(raw.substring(answerIdx - 80, answerIdx + 500));
}

// 5. Look for exposed global player instance
console.log("\n5. Global player exposure...");
const globalPlayer = raw.match(/window\.(player|quizPlayer|ISPQuizPlayer)\s*=/gi);
console.log("Global player assignments:", globalPlayer ? globalPlayer.length : 0);

// 6. Find the actual XML generation for quizReport (we saw this earlier)
const qrIdx = raw.indexOf("generateSessionXml");
if (qrIdx > 0) {
  console.log("\n6. XML Report generation (generateSessionXml)...");
  console.log(raw.substring(qrIdx - 50, qrIdx + 700));
}

console.log("\n=== END ANALYSIS ===");
