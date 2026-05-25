const fs = require("fs");
const raw = fs.readFileSync("contoh_soal/KIMIA_XII_UAS_2025 (Published)/data/player.js", "utf8");

console.log("=== DEEPER EXPERT DIG ===\n");

// Find how the main quiz object is created and stored
console.log("=== Finding main quiz / session model ===");
const quizModelMatches = raw.match(/new\s+\w+\s*\(\s*\{[^}]*quiz[^}]*\}/gi) || [];
console.log("Potential quiz model constructors:", quizModelMatches.length);

// Look for the object that has .evaluation() and .settings()
const evaluationPattern = /\.evaluation\s*\(\s*\)/g;
const evalMatches = raw.match(evaluationPattern);
console.log(".evaluation() calls:", evalMatches ? evalMatches.length : 0);

// Find where generateSessionXml is actually called
const callGenerate = raw.search(/generateSessionXml\s*\(/);
if (callGenerate > 0) {
  console.log("\n=== Where generateSessionXml is called ===");
  console.log(raw.substring(callGenerate - 200, callGenerate + 400));
}

// Look for the main player instance that gets passed to the callback
console.log("\n=== Player instance passed to user callback ===");
const callbackPattern = /ab\s*[:=]\s*function\s*\(\s*player|function\s*\(\s*player\s*\)\s*\{/i;
const cbMatch = raw.match(callbackPattern);
if (cbMatch) {
  console.log("Found player callback pattern");
}

// Search for how answers are set programmatically
console.log("\n=== Answer setting mechanisms ===");
const answerSet = raw.match(/\.setUserAnswer|\.applyAnswer|userAnswers\s*=|recordResponse/gi);
console.log("Answer setting methods:", answerSet ? answerSet.length : 0);

// Find the class that holds the quiz state (probably the one with generateSessionXml)
const vzClass = raw.match(/class\s+vz\s*\{[\s\S]{0,800}?generateSessionXml/gi);
if (vzClass) {
  console.log("\n=== vz class (report generator) ===");
  console.log(vzClass[0].substring(0, 900));
}

// Look for global exposure of the quiz object
console.log("\n=== Global exposure after init ===");
const windowAssign = raw.match(/window\.[A-Za-z0-9_]+\s*=\s*(quiz|player|model|session)/gi);
console.log("Window assignments:", windowAssign ? windowAssign.length : 0);

console.log("\n=== END DEEPER DIG ===");
