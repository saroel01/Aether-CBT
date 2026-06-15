<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import { toast } from '$lib/stores/toast';

  // Requirement 4.1-4.8: session CRUD + token + classes/rooms + effective status.
  let items: any[] = [];
  let examList: any[] = [];
  let classList: any[] = [];
  let roomList: any[] = [];

  let loading = true;
  let modalOpen = false;
  let editingId: number | null = null;
  let saving = false;
  let linkOpen = false;
  let linkSession: any = null;
  let selectedClasses: number[] = [];
  let selectedRooms: number[] = [];
  let linking = false;

  // Form state.
  let fExamID = 0;
  let fNama = '';
  let fMulai = '';   // datetime-local string
  let fSelesai = '';
  let fToken = '';
  let fStatus = 'draft';

  const STATUS_OPTIONS = [
    { value: 'draft', label: 'Draft' },
    { value: 'terjadwal', label: 'Terjadwal' },
    { value: 'aktif', label: 'Aktif' },
    { value: 'selesai', label: 'Selesai' },
    { value: 'dibatalkan', label: 'Dibatalkan' }
  ];

  function resetForm() {
    editingId = null;
    fExamID = 0;
    fNama = '';
    fMulai = '';
    fSelesai = '';
    fToken = '';
    fStatus = 'draft';
  }

  // Requirement 4.1: token auto-generate (unguessable).
  function generateToken(): string {
    const bytes = new Uint8Array(12);
    crypto.getRandomValues(bytes);
    return Array.from(bytes).map((b) => b.toString(16).padStart(2, '0')).join('');
  }

  // Convert a datetime-local string (e.g. 2026-06-15T08:00) into RFC3339 with local offset,
  // which the backend parses via time.Parse(time.RFC3339).
  function toRFC3339(local: string): string {
    if (!local) return '';
    const d = new Date(local);
    if (isNaN(d.getTime())) return '';
    return d.toISOString();
  }

  // The <input type="datetime-local"> value format is "YYYY-MM-DDTHH:MM" (no tz). Convert
  // from an ISO/RFC3339 timestamp (from the server, scanned as UTC) into that local input.
  function fromRFC3339ToLocal(iso: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return '';
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  // Effective-status heuristic for display: combines the administrative status with the
  // server-provided window. The authoritative check is server-side (Requirement 4.5); this
  // is only a UI hint.
  function effectiveBadge(s: any): { label: string; tone: string } {
    if (s.status === 'dibatalkan') return { label: 'Dibatalkan', tone: 'muted' };
    if (s.status === 'selesai') return { label: 'Selesai', tone: 'muted' };
    const now = Date.now();
    const mulai = new Date(s.waktu_mulai).getTime();
    const selesai = new Date(s.waktu_selesai).getTime();
    if (now < mulai) return { label: 'Belum dibuka', tone: 'warn' };
    if (now > selesai) return { label: 'Berakhir', tone: 'muted' };
    if (s.status === 'aktif' || s.status === 'terjadwal') return { label: 'Dapat dimasuki', tone: 'ok' };
    return { label: s.status, tone: 'muted' };
  }

  onMount(async () => {
    await loadAll();
  });

  async function loadAll() {
    loading = true;
    try {
      const [sessRes, examRes, classRes, roomRes] = await Promise.all([
        api('/admin/exam-sessions').catch(() => ({ data: [] })),
        api('/admin/exams').catch(() => ({ data: [] })),
        api('/classes').catch(() => ({ data: [] })),
        api('/rooms').catch(() => ({ data: [] }))
      ]);
      items = sessRes.data || [];
      examList = examRes.data || [];
      classList = classRes.data || [];
      roomList = roomRes.data || [];
    } catch {
      toast.error('Gagal memuat data sesi.');
    }
    loading = false;
  }

  function openCreate() {
    resetForm();
    fToken = generateToken();
    // Default window: today 08:00 → today 10:00 (local).
    const now = new Date();
    const pad = (n: number) => String(n).padStart(2, '0');
    const day = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
    fMulai = `${day}T08:00`;
    fSelesai = `${day}T10:00`;
    modalOpen = true;
  }

  function openEdit(s: any) {
    editingId = s.id;
    fExamID = s.exam_id;
    fNama = s.nama ?? '';
    fMulai = fromRFC3339ToLocal(s.waktu_mulai);
    fSelesai = fromRFC3339ToLocal(s.waktu_selesai);
    fToken = s.token;
    fStatus = s.status;
    modalOpen = true;
  }

  async function saveSession() {
    if (!fExamID) { toast.warning('Pilih ujian terlebih dahulu.'); return; }
    if (!fToken) { toast.warning('Token sesi wajib diisi.'); return; }
    const mulai = toRFC3339(fMulai);
    const selesai = toRFC3339(fSelesai);
    if (!mulai || !selesai) { toast.warning('Waktu mulai/selesai tidak valid.'); return; }
    const payload = {
      exam_id: Number(fExamID),
      nama: fNama || null,
      waktu_mulai: mulai,
      waktu_selesai: selesai,
      token: fToken,
      status: fStatus
    };
    saving = true;
    try {
      if (editingId) {
        await api(`/admin/exam-sessions/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
        toast.success('Sesi diperbarui.');
      } else {
        await api('/admin/exam-sessions', { method: 'POST', body: JSON.stringify(payload) });
        toast.success('Sesi dibuat.');
      }
      modalOpen = false;
      resetForm();
      await loadAll();
    } catch (e: any) {
      // 400 invalid window / 409 token overlap / 400 package required (Requirement 4.2-4.4).
      toast.error('Gagal menyimpan sesi: ' + e.message);
    }
    saving = false;
  }

  async function deleteSession(id: number, nama: string) {
    const confirm = window.confirm(`Hapus sesi "${nama || '#' + id}"?`);
    if (!confirm) return;
    try {
      await api(`/admin/exam-sessions/${id}`, { method: 'DELETE' });
      toast.success('Sesi dihapus.');
      await loadAll();
    } catch (e: any) {
      toast.error('Gagal menghapus sesi: ' + e.message);
    }
  }

  async function openLinker(s: any) {
    linkSession = s;
    // Best-effort: start from empty selection; the backend attaches additively (INSERT OR
    // IGNORE), so re-submitting existing links is a no-op.
    selectedClasses = [];
    selectedRooms = [];
    linkOpen = true;
  }

  async function linkParticipants() {
    if (!linkSession) return;
    linking = true;
    try {
      if (selectedClasses.length) {
        await api(`/admin/exam-sessions/${linkSession.id}/classes`, { method: 'POST', body: JSON.stringify({ ids: selectedClasses }) });
      }
      if (selectedRooms.length) {
        await api(`/admin/exam-sessions/${linkSession.id}/rooms`, { method: 'POST', body: JSON.stringify({ ids: selectedRooms }) });
      }
      toast.success('Peserta tertaut ke sesi.');
      linkOpen = false;
      await loadAll();
    } catch (e: any) {
      // 400 = cross-tenant id rejected (Requirement 4.7).
      toast.error('Gagal menautkan peserta: ' + e.message);
    }
    linking = false;
  }

  function examLabel(id: number): string {
    const e = examList.find((x) => x.id === id);
    if (!e) return '—';
    return e.nama || `Ujian #${e.id}`;
  }
</script>

<svelte:head>
  <title>Sesi Ujian - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <div class="border-b pb-6 flex items-end justify-between gap-4">
    <div>
      <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Sesi Ujian</h1>
      <p class="text-slate-500 text-sm">Gelombang terjadwal dengan jendela waktu, token unik, dan daftar kelas/ruang peserta.</p>
    </div>
    <Button variant="primary" theme="light" on:click={openCreate}>+ Buat Sesi</Button>
  </div>

  {#if loading}
    <div class="bg-white border rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
      <svg class="animate-spin h-6 w-6 text-indigo-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
      </svg>
      <span class="text-xs font-semibold text-slate-500">Memuat sesi...</span>
    </div>
  {:else}
    <Table>
      <thead>
        <tr>
          <th class="w-16">ID</th>
          <th>Nama / Ujian</th>
          <th>Token</th>
          <th>Jendela Waktu</th>
          <th class="w-32">Status Admin</th>
          <th class="w-32">Status Efektif</th>
          <th class="text-center w-40">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#each items as s}
          {@const eff = effectiveBadge(s)}
          <tr>
            <td class="font-mono text-slate-400 font-bold">{s.id}</td>
            <td>
              <div class="font-semibold text-slate-800">{s.nama || '—'}</div>
              <div class="text-xs text-slate-500">{examLabel(s.exam_id)}</div>
            </td>
            <td class="font-mono text-xs text-indigo-600 font-bold select-all">{s.token}</td>
            <td class="text-xs text-slate-500">
              <div>{new Date(s.waktu_mulai).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })}</div>
              <div>s/d {new Date(s.waktu_selesai).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })}</div>
            </td>
            <td><Badge>{s.status}</Badge></td>
            <td>
              <span class="text-xs font-bold px-2 py-1 rounded-full
                {eff.tone === 'ok' ? 'bg-emerald-50 text-emerald-700' : ''}
                {eff.tone === 'warn' ? 'bg-amber-50 text-amber-700' : ''}
                {eff.tone === 'muted' ? 'bg-slate-100 text-slate-500' : ''}">
                {eff.label}
              </span>
            </td>
            <td class="text-center">
              <div class="flex items-center justify-center gap-1.5">
                <Button variant="secondary" size="sm" theme="light" on:click={() => openLinker(s)}>Peserta</Button>
                <Button variant="ghost" size="sm" theme="light" on:click={() => openEdit(s)}>Sunting</Button>
                <Button variant="danger" size="sm" theme="light" on:click={() => deleteSession(s.id, s.nama)}>Hapus</Button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="7" class="text-center py-12 text-slate-400 font-medium">
              Belum ada sesi. Klik "+ Buat Sesi" untuk menjadwalkan.
            </td>
          </tr>
        {/each}
      </tbody>
    </Table>
  {/if}
</div>

<!-- Create / Edit session modal -->
<Modal bind:show={modalOpen} title={editingId ? 'Sunting Sesi' : 'Buat Sesi'} size="lg" theme="light">
  <div class="space-y-4">
    <div class="flex flex-col gap-2">
      <label for="sess_exam" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Ujian *</label>
      <select id="sess_exam" bind:value={fExamID} disabled={saving}
        class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold">
        <option value={0}>Pilih Ujian...</option>
        {#each examList as e}
          <option value={e.id}>{e.nama || 'Ujian #' + e.id} (mapel {e.mapel_id})</option>
        {/each}
      </select>
    </div>

    <Input id="sess_nama" label="Nama Sesi (opsional)" placeholder="Contoh: Gelombang 1" bind:value={fNama} disabled={saving} theme="light" />

    <div class="grid grid-cols-2 gap-4">
      <div class="flex flex-col gap-2">
        <label for="sess_mulai" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Waktu Mulai *</label>
        <input id="sess_mulai" type="datetime-local" bind:value={fMulai} disabled={saving}
          class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold" />
      </div>
      <div class="flex flex-col gap-2">
        <label for="sess_selesai" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Waktu Selesai *</label>
        <input id="sess_selesai" type="datetime-local" bind:value={fSelesai} disabled={saving}
          class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold" />
      </div>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <div class="flex flex-col gap-2">
        <label for="sess_token" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Token Sesi *</label>
        <div class="flex gap-2">
          <input id="sess_token" bind:value={fToken} disabled={saving}
            class="flex-1 h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-mono font-bold" />
          <Button variant="secondary" size="sm" theme="light" on:click={() => (fToken = generateToken())} disabled={saving}>Acak</Button>
        </div>
      </div>
      <div class="flex flex-col gap-2">
        <label for="sess_status" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Status</label>
        <select id="sess_status" bind:value={fStatus} disabled={saving}
          class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold">
          {#each STATUS_OPTIONS as o}
            <option value={o.value}>{o.label}</option>
          {/each}
        </select>
      </div>
    </div>
    <p class="text-[11px] text-slate-400 leading-relaxed">
      Catatan: transisi ke <strong>terjadwal/aktif</strong> ditolak bila ujian belum menautkan paket soal (Requirement 4.3).
    </p>
  </div>

  <div slot="footer" class="flex items-center justify-end gap-3 w-full">
    <Button variant="ghost" size="sm" theme="light" on:click={() => { modalOpen = false; resetForm(); }} disabled={saving}>Batal</Button>
    <Button variant="primary" size="sm" theme="light" on:click={saveSession} loading={saving}>{editingId ? 'Simpan Perubahan' : 'Buat Sesi'}</Button>
  </div>
</Modal>

<!-- Link participants modal -->
<Modal bind:show={linkOpen} title={`Tautkan Peserta — ${linkSession?.nama || '#' + (linkSession?.id ?? '')}`} size="lg" theme="light">
  <div class="space-y-5">
    <div class="flex flex-col gap-2">
      <span class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Kelas Peserta</span>
      <div class="grid grid-cols-2 gap-2 max-h-44 overflow-y-auto p-2 border border-slate-100 rounded-2xl bg-slate-50/40">
        {#each classList as c}
          <label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer p-1">
            <input type="checkbox" value={c.id} bind:group={selectedClasses} class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600/20" />
            <span class="font-semibold">{c.nama_kelas}</span>
            {#if c.tingkat}<span class="text-[10px] text-slate-400">{c.tingkat}</span>{/if}
          </label>
        {:else}
          <span class="text-xs text-slate-400 col-span-2 p-2">Belum ada kelas.</span>
        {/each}
      </div>
    </div>
    <div class="flex flex-col gap-2">
      <span class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Ruang Peserta (opsional, batasi peserta)</span>
      <div class="grid grid-cols-2 gap-2 max-h-44 overflow-y-auto p-2 border border-slate-100 rounded-2xl bg-slate-50/40">
        {#each roomList as r}
          <label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer p-1">
            <input type="checkbox" value={r.id} bind:group={selectedRooms} class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600/20" />
            <span class="font-semibold">{r.nama_ruang}</span>
          </label>
        {:else}
          <span class="text-xs text-slate-400 col-span-2 p-2">Belum ada ruang.</span>
        {/each}
      </div>
      <p class="text-[11px] text-slate-400">Bila ruang dipilih, hanya peserta di ruang tersebut yang berhak (Requirement 5.2).</p>
    </div>
  </div>
  <div slot="footer" class="flex items-center justify-end gap-3 w-full">
    <Button variant="ghost" size="sm" theme="light" on:click={() => (linkOpen = false)} disabled={linking}>Tutup</Button>
    <Button variant="primary" size="sm" theme="light" on:click={linkParticipants} loading={linking}>Tautkan Terpilih</Button>
  </div>
</Modal>
