<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, apiUrl, authHeaders } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Timer from '$lib/components/ui/Timer.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import { toast } from '$lib/stores/toast';

  let pesertaId = '';
  let pesertaNoId = '';
  let examToken = '';
  let attemptToken = '';
  let selectedMapelId = '';
  let selectedMapelName = '';

  let duration = 3600; // 1 Hour in seconds
  let activeQuestionIndex = 0;
  let showConfirmSubmit = false;
  let showResultModal = false;
  let loading = false;

  // Anti-cheat state tracking
  let tabSwitchCount = 0;
  let showCheatModal = false;

  interface Question {
    id: string;
    text: string;
    options: { key: string; text: string }[];
    correctAnswer: string;
    selectedAnswer?: string;
  }

  let questions: Question[] = [];
  let score = 0;

  // Personalize questions based on subject
  function generateQuestions(subjectName: string): Question[] {
    const list: Question[] = [];
    const isMath = subjectName.toLowerCase().includes('matematika');
    const isEnglish = subjectName.toLowerCase().includes('inggris');
    const isPhysics = subjectName.toLowerCase().includes('fisika');

    if (isMath) {
      list.push(
        { id: 'Q1', text: 'Jika f(x) = 3x + 5, berapakah nilai dari f(4)?', options: [{ key: 'A', text: '12' }, { key: 'B', text: '15' }, { key: 'C', text: '17' }, { key: 'D', text: '20' }, { key: 'E', text: '23' }], correctAnswer: 'C' },
        { id: 'Q2', text: 'Berapakah nilai x yang memenuhi persamaan logaritma ^2log(x) = 5?', options: [{ key: 'A', text: '10' }, { key: 'B', text: '16' }, { key: 'C', text: '25' }, { key: 'D', text: '32' }, { key: 'E', text: '64' }], correctAnswer: 'D' },
        { id: 'Q3', text: 'Tentukan turunan pertama dari fungsi f(x) = 4x^3 - 2x^2 + x.', options: [{ key: 'A', text: '12x^2 - 4x + 1' }, { key: 'B', text: '12x^2 - 4x' }, { key: 'C', text: '8x^2 - 2x + 1' }, { key: 'D', text: '4x^2 - 4x' }, { key: 'E', text: '12x^3 - 4x^2' }], correctAnswer: 'A' },
        { id: 'Q4', text: 'Sebuah segitiga siku-siku memiliki alas 6 cm dan tinggi 8 cm. Berapakah panjang hipotenusanya?', options: [{ key: 'A', text: '9 cm' }, { key: 'B', text: '10 cm' }, { key: 'C', text: '12 cm' }, { key: 'D', text: '14 cm' }, { key: 'E', text: '15 cm' }], correctAnswer: 'B' },
        { id: 'Q5', text: 'Berapakah jumlah deret geometri tak hingga dari 12 + 6 + 3 + 1.5 + ...?', options: [{ key: 'A', text: '20' }, { key: 'B', text: '22' }, { key: 'C', text: '24' }, { key: 'D', text: '26' }, { key: 'E', text: '28' }], correctAnswer: 'C' }
      );
    } else if (isPhysics) {
      list.push(
        { id: 'Q1', text: 'Sebuah mobil bergerak dengan kecepatan konstan 20 m/s selama 10 detik. Berapakah jarak yang ditempuh mobil tersebut?', options: [{ key: 'A', text: '100 m' }, { key: 'B', text: '150 m' }, { key: 'C', text: '200 m' }, { key: 'D', text: '250 m' }, { key: 'E', text: '300 m' }], correctAnswer: 'C' },
        { id: 'Q2', text: 'Menurut Hukum II Newton, gaya berbanding lurus dengan...', options: [{ key: 'A', text: 'Waktu' }, { key: 'B', text: 'Kecepatan' }, { key: 'C', text: 'Massa dan Percepatan' }, { key: 'D', text: 'Jarak' }, { key: 'E', text: 'Usaha' }], correctAnswer: 'C' },
        { id: 'Q3', text: 'Berapakah energi kinetik dari benda bermassa 2 kg yang bergerak dengan kecepatan 4 m/s?', options: [{ key: 'A', text: '8 Joule' }, { key: 'B', text: '12 Joule' }, { key: 'C', text: '16 Joule' }, { key: 'D', text: '20 Joule' }, { key: 'E', text: '32 Joule' }], correctAnswer: 'C' },
        { id: 'Q4', text: 'Alat yang digunakan untuk mengukur kuat arus listrik adalah...', options: [{ key: 'A', text: 'Voltmeter' }, { key: 'B', text: 'Ohmmeter' }, { key: 'C', text: 'Amperemeter' }, { key: 'D', text: 'Termometer' }, { key: 'E', text: 'Barometer' }], correctAnswer: 'C' },
        { id: 'Q5', text: 'Berapakah besar hambatan listrik pengganti dari dua resistor 4 Ohm yang dirangkai secara paralel?', options: [{ key: 'A', text: '2 Ohm' }, { key: 'B', text: '4 Ohm' }, { key: 'C', text: '8 Ohm' }, { key: 'D', text: '12 Ohm' }, { key: 'E', text: '16 Ohm' }], correctAnswer: 'A' }
      );
    } else if (isEnglish) {
      list.push(
        { id: 'Q1', text: 'Complete the sentence: "If I _____ his number, I would call him."', options: [{ key: 'A', text: 'know' }, { key: 'B', text: 'knew' }, { key: 'C', text: 'known' }, { key: 'D', text: 'knowing' }, { key: 'E', text: 'have known' }], correctAnswer: 'B' },
        { id: 'Q2', text: 'What is the synonym of the word "Meticulous"?', options: [{ key: 'A', text: 'Careless' }, { key: 'B', text: 'Extremely careful' }, { key: 'C', text: 'Fast' }, { key: 'D', text: 'Messy' }, { key: 'E', text: 'Slow' }], correctAnswer: 'B' },
        { id: 'Q3', text: 'Identify the passive form: "The chef cooked a delicious meal."', options: [{ key: 'A', text: 'A delicious meal is cooked by the chef.' }, { key: 'B', text: 'A delicious meal was cooked by the chef.' }, { key: 'C', text: 'A delicious meal has been cooked by the chef.' }, { key: 'D', text: 'A delicious meal was cooking by the chef.' }, { key: 'E', text: 'A delicious meal cooks the chef.' }], correctAnswer: 'B' },
        { id: 'Q4', text: 'Which word is an antonym of "Generous"?', options: [{ key: 'A', text: 'Kind' }, { key: 'B', text: 'Helpful' }, { key: 'C', text: 'Stingy' }, { key: 'D', text: 'Happy' }, { key: 'E', text: 'Friendly' }], correctAnswer: 'C' },
        { id: 'Q5', text: 'Choose the correct preposition: "She is highly capable _____ resolving the issue."', options: [{ key: 'A', text: 'to' }, { key: 'B', text: 'for' }, { key: 'C', text: 'of' }, { key: 'D', text: 'at' }, { key: 'E', text: 'with' }], correctAnswer: 'C' }
      );
    } else {
      list.push(
        { id: 'Q1', text: 'Siapakah presiden pertama Republik Indonesia?', options: [{ key: 'A', text: 'Drs. Mohammad Hatta' }, { key: 'B', text: 'Ir. Soekarno' }, { key: 'C', text: 'Soeharto' }, { key: 'D', text: 'B.J. Habibie' }, { key: 'E', text: 'Gus Dur' }], correctAnswer: 'B' },
        { id: 'Q2', text: 'Lambang sila ketiga Pancasila adalah...', options: [{ key: 'A', text: 'Bintang' }, { key: 'B', text: 'Rantai' }, { key: 'C', text: 'Pohon Beringin' }, { key: 'D', text: 'Kepala Banteng' }, { key: 'E', text: 'Padi dan Kapas' }], correctAnswer: 'C' },
        { id: 'Q3', text: 'Benua terkecil di dunia adalah...', options: [{ key: 'A', text: 'Asia' }, { key: 'B', text: 'Afrika' }, { key: 'C', text: 'Eropa' }, { key: 'D', text: 'Amerika' }, { key: 'E', text: 'Australia' }], correctAnswer: 'E' },
        { id: 'Q4', text: 'Zat hijau daun pada tumbuhan disebut dengan...', options: [{ key: 'A', text: 'Stomata' }, { key: 'B', text: 'Klorofil' }, { key: 'C', text: 'Xilem' }, { key: 'D', text: 'Floem' }, { key: 'E', text: 'Spora' }], correctAnswer: 'B' },
        { id: 'Q5', text: 'Samudra terluas di dunia adalah...', options: [{ key: 'A', text: 'Samudra Hindia' }, { key: 'B', text: 'Samudra Atlantik' }, { key: 'C', text: 'Samudra Arktik' }, { key: 'D', text: 'Samudra Pasifik' }, { key: 'E', text: 'Samudra Antartika' }], correctAnswer: 'D' }
      );
    }

    // Expand to 20 questions by repeating and adjusting IDs to make it a full scale test
    const finalQuestions: Question[] = [];
    for (let i = 0; i < 20; i++) {
      const template = list[i % list.length];
      finalQuestions.push({
        id: `Q${i + 1}`,
        text: `[Soal No ${i + 1}] ${template.text}`,
        options: template.options,
        correctAnswer: template.correctAnswer
      });
    }
    return finalQuestions;
  }

  onMount(async () => {
    pesertaId = localStorage.getItem('peserta_id') || '';
    pesertaNoId = localStorage.getItem('peserta_no_id') || '';
    examToken = localStorage.getItem('exam_token') || '';
    selectedMapelId = localStorage.getItem('selected_mapel_id') || '';
    selectedMapelName = localStorage.getItem('selected_mapel_name') || 'Mata Pelajaran';
    attemptToken = localStorage.getItem('attempt_token') || '';

    if (!pesertaId || !selectedMapelId || !attemptToken) {
      toast.error('Silakan login dan pilih mata pelajaran terlebih dahulu.');
      window.location.href = '/student/login';
      return;
    }

    questions = generateQuestions(selectedMapelName);

    // Anti-cheat tab switch monitoring
    if (typeof window !== 'undefined') {
      window.addEventListener('blur', recordTabSwitch);
      document.addEventListener('visibilitychange', handleVisibilityChange);
    }
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('blur', recordTabSwitch);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    }
  });

  async function recordTabSwitch() {
    if (showResultModal || showConfirmSubmit) return; // ignore if already finished or submitting

    tabSwitchCount++;
    showCheatModal = true;
    toast.error(`⚠️ Peringatan Keamanan! Dilarang meninggalkan halaman ujian! (${tabSwitchCount}x)`);

    try {
      await api('/student/infraction', {
        method: 'POST',
        body: JSON.stringify({
          peserta_id: parseInt(pesertaId),
          mapel_id: parseInt(selectedMapelId)
        })
      });
    } catch {}
  }

  function handleVisibilityChange() {
    if (document.hidden) {
      recordTabSwitch();
    }
  }

  function selectOption(optKey: string) {
    questions[activeQuestionIndex].selectedAnswer = optKey;
    // force update Svelte array binding
    questions = [...questions];
    reportProgress();
  }

  async function reportProgress() {
    try {
      const activeAnswered = questions.filter(q => q.selectedAnswer !== undefined).length;
      await api('/student/progress', {
        method: 'POST',
        body: JSON.stringify({
          peserta_id: parseInt(pesertaId),
          mapel_id: parseInt(selectedMapelId),
          answered_count: activeAnswered,
          total_questions: questions.length
        })
      });
    } catch {}
  }

  function handleTimeExpired() {
    toast.warning('Waktu ujian telah habis! Jawaban Anda dikirim secara otomatis.');
    submitExamResult();
  }

  function escapeXml(value: string): string {
    return value
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&apos;');
  }

  function optionIndex(q: Question, key: string | undefined): number {
    if (!key) return -1;
    return q.options.findIndex((opt) => opt.key === key);
  }

  // Generates a compact iSpring-compatible quizReport XML for the built-in simulator.
  // Real iSpring packages send the same report through the `dr` POST field.
  function generateDetailedXML(): string {
    const earnedPoints = score * 5;
    const percent = Math.round((earnedPoints / 100) * 100);
    const passed = percent >= 70;
    let xml = `<?xml version="1.0" encoding="UTF-8"?>\n`;
    xml += `<quizReport xmlns="http://www.ispringsolutions.com/ispring/quizbuilder/quizresults" version="9">\n`;
    xml += `  <quizSettings>\n`;
    xml += `    <passingPercent>70</passingPercent>\n`;
    xml += `  </quizSettings>\n`;
    xml += `  <summary passed="${passed ? 'true' : 'false'}" percent="${percent}" finishTimestamp="${new Date().toISOString()}" />\n`;
    xml += `  <questions>\n`;
    
    questions.forEach((q) => {
      const isCorrect = q.selectedAnswer === q.correctAnswer;
      const status = q.selectedAnswer === undefined ? 'notAnswered' : isCorrect ? 'correct' : 'incorrect';
      const points = isCorrect ? 5 : 0;
      const userAnswerIndex = optionIndex(q, q.selectedAnswer);
      const correctAnswerIndex = optionIndex(q, q.correctAnswer);
      
      xml += `    <multipleChoiceQuestion id="${escapeXml(q.id)}" evaluationEnabled="true" maxPoints="5" maxAttempts="1" usedAttempts="1" awardedPoints="${points}" status="${status}">\n`;
      xml += `      <direction><text>${escapeXml(q.text)}</text></direction>\n`;
      xml += `      <answers correctAnswerIndex="${correctAnswerIndex}"${userAnswerIndex >= 0 ? ` userAnswerIndex="${userAnswerIndex}"` : ''}>\n`;
      q.options.forEach((opt) => {
        xml += `        <answer><text>${escapeXml(opt.text)}</text></answer>\n`;
      });
      xml += `      </answers>\n`;
      xml += `    </multipleChoiceQuestion>\n`;
    });
    
    xml += `  </questions>\n`;
    xml += `  <groups />\n`;
    xml += `</quizReport>`;
    return xml;
  }

  async function submitExamResult() {
    loading = true;
    showConfirmSubmit = false;

    // Calculate score
    let correctCount = 0;
    questions.forEach((q) => {
      if (q.selectedAnswer === q.correctAnswer) {
        correctCount++;
      }
    });

    score = correctCount; // total correct questions
    const earnedPoints = correctCount * 5; // e.g. 85.00
    const detailedXML = generateDetailedXML();

    try {
      // Send result to iSpring Webhook using Form urlencoded as required by iSpring standard receiver!
      const formData = new URLSearchParams();
      formData.append('sid', pesertaNoId);
      formData.append('sp', earnedPoints.toString());
      formData.append('tp', '100');
      formData.append('dr', detailedXML);
      formData.append('attempt_token', attemptToken);

      // Perform HTTP POST to the webhook endpoint
      const res = await fetch(apiUrl('/ispring/webhook'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          ...authHeaders()
        },
        body: formData.toString()
      });

      if (res.ok) {
        toast.success('Ujian berhasil diselesaikan! Data hasil tes terkirim.');
        showResultModal = true;
      } else {
        throw new Error('Server returned error status');
      }
    } catch (e: any) {
      toast.error('Gagal mengirimkan hasil ujian. Silakan coba kembali.');
    }
    loading = false;
  }

  function finishExam() {
    localStorage.clear();
    window.location.href = '/student/login';
  }
</script>

<svelte:head>
  <title>Lembar Ujian: {selectedMapelName} - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-slate-950 bg-grid-sovereign text-slate-100 flex flex-col justify-between select-none relative overflow-hidden">
  <!-- Top focus-mode bar (Ramping, minimal, Outfit typography) -->
  <header class="border-b border-slate-900 bg-slate-950/80 backdrop-blur-md px-6 py-4 flex justify-between items-center z-10 sticky top-0">
    <div class="flex items-center gap-4">
      <div>
        <span class="text-[10px] uppercase tracking-widest text-indigo-500 font-bold font-mono">Ujian Sedang Berlangsung</span>
        <h1 class="text-lg font-bold text-slate-200 font-display">{selectedMapelName}</h1>
      </div>
    </div>

    <!-- Active Timer -->
    <div class="flex items-center gap-6">
      <Timer durationSeconds={duration} on:expired={handleTimeExpired} />
      <Button 
        variant="danger" 
        size="sm" 
        class="font-semibold" 
        on:click={() => showConfirmSubmit = true}
      >
        Hentikan Ujian
      </Button>
    </div>
  </header>

  <!-- Main layout grid (Sovereign 65-75ch Reading Zone & Sidebar Navigation) -->
  <div class="flex-1 max-w-6xl w-full mx-auto p-4 md:p-6 grid grid-cols-1 lg:grid-cols-4 gap-6 z-10 overflow-hidden">
    <!-- Active Question (3/4 Grid) -->
    <div class="lg:col-span-3 flex flex-col h-full gap-4">
      {#if questions.length > 0}
        {@const activeQ = questions[activeQuestionIndex]}
        <Card theme="dark" padding="lg" class="border-slate-900 bg-slate-900/40 text-slate-100 shadow-xl flex-1 flex flex-col justify-between relative overflow-hidden min-h-[450px]">
          <!-- Top subtle decoration line -->
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-800"></div>

          <div>
            <!-- Question indicator -->
            <div class="flex items-center justify-between mb-6 border-b border-slate-900 pb-3">
              <span class="text-[10px] font-bold text-slate-500 tracking-widest font-mono">PERTANYAAN NO {activeQuestionIndex + 1}</span>
              <span class="text-[10px] px-2.5 py-0.5 bg-indigo-950/50 text-indigo-400 border border-indigo-900/40 rounded-lg font-bold font-mono tracking-wider">PILIHAN GANDA</span>
            </div>

            <!-- Sovereign Reading Zone: generous leading-relaxed and line-width capped for premium readability -->
            <div class="max-w-[70ch] leading-relaxed mb-8">
              <p class="text-lg md:text-xl font-medium text-slate-200 leading-relaxed">
                {activeQ.text}
              </p>
            </div>

            <!-- Tactile Options with Inset Neumorphic State when Selected -->
            <div class="space-y-3 max-w-[72ch]">
              {#each activeQ.options as opt}
                {@const isSelected = activeQ.selectedAnswer === opt.key}
                <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
                <div 
                  class="flex items-center gap-4 px-5 py-4 rounded-2xl border outline-none cursor-pointer transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] select-none
                    {isSelected ? 'bg-slate-950/80 border-indigo-500/80 text-indigo-300 shadow-tactile-inset scale-[0.99]' : 'bg-slate-900/40 border-slate-900 hover:border-slate-800 text-slate-300 hover:scale-[1.005]'}"
                  on:click={() => selectOption(opt.key)}
                >
                  <!-- Tactile box indicating answer index -->
                  <div class="h-9 w-9 flex items-center justify-center rounded-xl font-bold font-mono text-sm border transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]
                    {isSelected ? 'bg-indigo-600 border-indigo-400 text-white shadow-md shadow-indigo-600/20' : 'bg-slate-950/40 border-slate-850 text-slate-500'}"
                  >
                    {opt.key}
                  </div>
                  <span class="text-base font-medium">{opt.text}</span>
                </div>
              {/each}
            </div>
          </div>

          <!-- Bottom navigation buttons -->
          <div class="flex items-center justify-between pt-8 border-t border-slate-900 mt-8 gap-4">
            <Button 
              variant="secondary" 
              size="md" 
              disabled={activeQuestionIndex === 0} 
              on:click={() => activeQuestionIndex -= 1}
            >
              Sebelumnya
            </Button>
            
            <div class="text-xs font-bold text-slate-500 font-mono tracking-widest">
              {activeQuestionIndex + 1} / {questions.length}
            </div>

            {#if activeQuestionIndex < questions.length - 1}
              <Button 
                variant="primary" 
                size="md" 
                on:click={() => activeQuestionIndex += 1}
              >
                Berikutnya
              </Button>
            {:else}
              <Button 
                variant="primary" 
                size="md" 
                class="bg-indigo-600 hover:bg-indigo-700 text-white border-none"
                on:click={() => showConfirmSubmit = true}
              >
                Selesaikan Ujian
              </Button>
            {/if}
          </div>
        </Card>
      {/if}
    </div>

    <!-- Right grid navigation (1/4 Grid) -->
    <div class="lg:col-span-1 flex flex-col gap-6">
      <Card theme="dark" padding="md" class="border-slate-900 bg-slate-900/40 text-slate-100 shadow-xl flex flex-col justify-between h-full min-h-[400px]">
        <div>
          <h3 class="text-[10px] font-bold uppercase tracking-widest text-slate-500 mb-5 pb-2 border-b border-slate-900 font-mono">
            Peta Soal Ujian
          </h3>

          <!-- Grid Navigation representing sovereign colors -->
          <div class="grid grid-cols-5 gap-2">
            {#each questions as q, index}
              {@const isActive = index === activeQuestionIndex}
              {@const isAnswered = q.selectedAnswer !== undefined}
              
              <button 
                type="button"
                class="h-10 flex items-center justify-center font-mono font-bold text-xs rounded-xl border transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] outline-none
                  {isActive ? 'bg-indigo-950/40 border-indigo-500/80 text-indigo-400 shadow-inner' : 
                   isAnswered ? 'bg-emerald-950/20 border-emerald-900/50 text-emerald-400' : 
                   'bg-slate-950/30 border-slate-900/80 text-slate-600 hover:border-slate-800'}"
                on:click={() => activeQuestionIndex = index}
              >
                {index + 1}
              </button>
            {/each}
          </div>
        </div>

        <!-- Dynamic legend -->
        <div class="pt-6 border-t border-slate-900 mt-6 space-y-3">
          <div class="text-[10px] font-bold text-slate-500 uppercase tracking-widest font-mono">Legenda Status</div>
          <div class="flex flex-col gap-2.5">
            <div class="flex items-center gap-2.5 text-xs text-slate-400 font-semibold">
              <div class="h-4 w-4 bg-slate-950/30 border border-slate-900 rounded-md"></div>
              <span>Belum dikerjakan</span>
            </div>
            <div class="flex items-center gap-2.5 text-xs text-slate-400 font-semibold">
              <div class="h-4 w-4 bg-emerald-950/20 border border-emerald-900/50 rounded-md"></div>
              <span>Sudah dijawab</span>
            </div>
            <div class="flex items-center gap-2.5 text-xs text-slate-400 font-semibold">
              <div class="h-4 w-4 bg-indigo-950/40 border border-indigo-500/80 rounded-md"></div>
              <span>Soal aktif</span>
            </div>
          </div>
        </div>
      </Card>
    </div>
  </div>

  <!-- Confirm submit modal (Clean and sovereign) -->
  <Modal theme="dark" show={showConfirmSubmit} title="Selesaikan Sesi Ujian" size="sm">
    <div class="text-slate-300 p-2">
      <p class="text-lg font-bold text-slate-100 mb-2 font-display">Apakah Anda yakin ingin menyudahi ujian?</p>
      <p class="text-sm text-slate-400 leading-relaxed">
        Menekan tombol kirim akan mengunci seluruh lembar jawaban Anda dan melaporkannya secara instan ke server sekolah. Anda tidak dapat melakukan koreksi jawaban kembali setelah ini.
      </p>
    </div>
    <div slot="footer" class="flex gap-3 justify-end">
      <Button variant="secondary" size="sm" on:click={() => showConfirmSubmit = false} disabled={loading}>Batal</Button>
      <Button variant="primary" size="sm" on:click={submitExamResult} {loading}>Kirim Jawaban</Button>
    </div>
  </Modal>

  <!-- Complete report modal (Elegant & calming) -->
  <Modal theme="dark" show={showResultModal} title="Sesi Ujian Selesai" size="sm">
    <div class="text-center py-6 text-slate-300 px-4">
      <div class="h-16 w-16 bg-emerald-950/20 text-emerald-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-emerald-900/30 shadow-sm">
        <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
        </svg>
      </div>
      <h3 class="text-2xl font-bold text-slate-100 mb-2 font-display">Lembar Jawaban Dikirim!</h3>
      <p class="text-sm text-slate-400 leading-relaxed mb-6">
        Terima kasih telah berpartisipasi.<br>Data pengerjaan Anda telah dilaporkan dan tercatat dengan aman pada pangkalan data utama sekolah.
      </p>
      
      <div class="bg-slate-950/40 border border-[oklch(0.22_0.016_250)] p-4 rounded-2xl max-w-xs mx-auto mb-6 flex justify-between text-left text-sm">
        <span class="text-slate-400 font-semibold">Mata Pelajaran:</span>
        <span class="font-bold text-slate-200">{selectedMapelName}</span>
      </div>

      <Button variant="primary" size="md" class="w-full" on:click={finishExam}>
        Selesai & Keluar Sesi
      </Button>
    </div>
  </Modal>

  <!-- Anti-Cheat Infraction Warning Overlay Modal (Soothing but firm) -->
  <Modal theme="dark" show={showCheatModal && tabSwitchCount < 3} title="Peringatan Keamanan Ujian" size="sm">
    <div class="text-center py-4 text-slate-300 px-4">
      <div class="h-16 w-16 bg-red-950/20 text-red-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-red-900/30 animate-pulse">
        <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      </div>
      <h3 class="text-xl font-bold text-red-500 mb-2 font-display">Peringatan Keamanan!</h3>
      <p class="text-sm text-slate-400 leading-relaxed mb-5">
        Sistem mencatat aktivitas perpindahan halaman (meninggalkan tab ujian aktif, membuka aplikasi lain, atau meminimalkan browser). Aktivitas ini telah dilaporkan kepada proktor.
      </p>
      
      <div class="bg-red-950/30 border border-red-900/20 p-3 rounded-2xl max-w-xs mx-auto mb-6 text-sm text-red-400 font-bold">
        Jumlah Pelanggaran: {tabSwitchCount}x
      </div>

      <Button variant="primary" size="md" class="w-full" on:click={() => showCheatModal = false}>
        Kembali Mengerjakan Ujian
      </Button>
    </div>
  </Modal>

  <!-- Authoritative Lock overlay (Extreme premium full screen block if infractions reach 3) -->
  {#if tabSwitchCount >= 3}
    <div class="fixed inset-0 bg-slate-950/95 backdrop-blur-md flex items-center justify-center z-50 p-4 select-none">
      <div class="bg-slate-900 border border-red-900/30 p-8 rounded-3xl max-w-md w-full text-center shadow-2xl space-y-6 relative overflow-hidden">
        <!-- Accent top border -->
        <div class="absolute top-0 left-0 w-full h-[2px] bg-red-600"></div>

        <div class="h-16 w-16 bg-red-950/40 text-red-400 rounded-2xl flex items-center justify-center mx-auto border border-red-900/30 animate-pulse">
          <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        </div>
        
        <div class="space-y-2">
          <h3 class="text-2xl font-extrabold text-red-500 font-display">UJIAN ANDA DIKUNCI!</h3>
          <p class="text-slate-400 text-sm leading-relaxed">
            Sesi ujian Anda telah ditangguhkan secara otomatis oleh sistem karena terdeteksi meninggalkan layar ujian sebanyak <strong>{tabSwitchCount} kali</strong>.
          </p>
        </div>
        
        <div class="bg-red-950/10 border border-red-900/20 p-4 rounded-2xl text-xs text-red-400 leading-normal font-semibold">
          Silakan hubungi proktor / pengawas ruangan Anda di meja pengawasan untuk memverifikasi dan membuka kembali sesi ujian Anda.
        </div>
      </div>
    </div>
  {/if}
</div>
