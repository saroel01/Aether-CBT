import json
import sqlite3
import subprocess
import time
import urllib.request
import urllib.error
import sys
import os

RELEASE_DIR = os.path.abspath('release-test')
DB_PATH = os.path.join(RELEASE_DIR, 'data', 'cbt_aether.db')
EXE_PATH = os.path.join(RELEASE_DIR, 'aether-cbt.exe')
BASE_URL = 'http://127.0.0.1:3000'

def http_post(path, data, headers=None):
    url = f'{BASE_URL}{path}'
    body = json.dumps(data).encode('utf-8')
    req_headers = {'Content-Type': 'application/json', 'X-Tenant-ID': '1'}
    if headers:
        req_headers.update(headers)
    req = urllib.request.Request(url, data=body, headers=req_headers)
    try:
        with urllib.request.urlopen(req) as resp:
            resp_headers = dict(resp.headers)
            body_content = resp.read().decode('utf-8')
            try:
                parsed = json.loads(body_content)
            except Exception:
                parsed = body_content
            return resp.status, parsed, resp_headers
    except urllib.error.HTTPError as e:
        body_content = e.read().decode('utf-8')
        try:
            parsed = json.loads(body_content)
        except Exception:
            parsed = body_content
        return e.code, parsed, dict(e.headers)

def http_get(path, headers=None):
    url = f'{BASE_URL}{path}'
    req_headers = {'X-Tenant-ID': '1'}
    if headers:
        req_headers.update(headers)
    req = urllib.request.Request(url, headers=req_headers)
    try:
        with urllib.request.urlopen(req) as resp:
            resp_headers = dict(resp.headers)
            body_content = resp.read().decode('utf-8', errors='ignore')
            try:
                parsed = json.loads(body_content)
            except Exception:
                parsed = body_content
            return resp.status, parsed, resp_headers
    except urllib.error.HTTPError as e:
        body_content = e.read().decode('utf-8', errors='ignore')
        try:
            parsed = json.loads(body_content)
        except Exception:
            parsed = body_content
        return e.code, parsed, dict(e.headers)

print('[TEST] Starting release-test/aether-cbt.exe...')
proc = subprocess.Popen([EXE_PATH], cwd=RELEASE_DIR, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

try:
    # 1. Wait for server ready
    ready = False
    for i in range(25):
        try:
            status, _, _ = http_get('/api/health')
            if status == 200:
                ready = True
                print(f'[TEST] Server ready (probed /api/health -> status {status})')
                break
        except Exception:
            pass
        time.sleep(0.5)

    if not ready:
        raise RuntimeError('Server did not start within 12.5 seconds')

    # 2. Test student login with 2026001 / siswa123 / AETHER1
    print('[TEST] 1. Testing student login for 2026001 (Ahmad Fauzi)...')
    status, res, _ = http_post('/api/auth/student-login', {
        'no_id': '2026001',
        'password': 'siswa123',
        'token': 'AETHER1'
    })
    print(f'   Status: {status}')
    assert status == 200, f'Expected 200, got {status}'
    assert res.get('success') is True, f'Expected success=True'
    assert res.get('data', {}).get('peserta_id') == 1, 'Expected peserta_id=1'
    assert res.get('data', {}).get('session_id') == 1, 'Expected session_id=1'
    assert res.get('data', {}).get('exam_id') == 1, 'Expected exam_id=1'
    student_jwt = res.get('data', {}).get('token', '')
    assert len(student_jwt) > 20, 'Expected valid JWT token'
    assert res.get('data', {}).get('user', {}).get('role') == 'student'
    assert res.get('data', {}).get('user', {}).get('username') == '2026001'
    print('   [PASS] Student 2026001 login succeeded with valid JWT and session data!')

    # 3. Test all 10 demo students
    print('[TEST] 2. Testing all 10 demo students (2026001 - 2026010)...')
    for idx in range(1, 11):
        no_id = f'2026{idx:03d}'
        st, r, _ = http_post('/api/auth/student-login', {
            'no_id': no_id,
            'password': 'siswa123',
            'token': 'AETHER1'
        })
        assert st == 200 and r.get('success') is True, f'Failed for student {no_id}: {st} {r}'
        assert r.get('data', {}).get('peserta_id') == idx, f'Mismatch id for student {no_id}'
        print(f'   - Student {no_id} (ID: {idx}): PASS')
    print('   [PASS] All 10 demo students authenticated successfully!')

    # 4. Test student authenticated exam flow: /student/my-sessions -> /student/start -> /exam/content
    print('[TEST] 3. Testing full student exam session start and content delivery...')
    student_headers = {'Authorization': f'Bearer {student_jwt}'}
    st_info, r_info, _ = http_get('/api/student/active-info', headers=student_headers)
    assert st_info == 200 and r_info.get('success') is True, f'active-info failed: {st_info} {r_info}'
    print('   - /api/student/active-info: PASS')

    st_sess, r_sess, _ = http_get('/api/student/my-sessions', headers=student_headers)
    assert st_sess == 200 and r_sess.get('success') is True, f'my-sessions failed: {st_sess} {r_sess}'
    sessions = r_sess.get('data', [])
    assert len(sessions) > 0, 'Expected at least one session'
    assert sessions[0].get('id') == 1, 'Expected session id 1'
    assert sessions[0].get('enterable') is True, 'Expected session to be enterable'
    print(f'   - /api/student/my-sessions: PASS (Session #{sessions[0].get("id")}, enterable={sessions[0].get("enterable")})')

    # Start the exam session
    st_start, r_start, start_headers = http_post('/api/student/start', {
        'peserta_id': 1,
        'session_id': 1
    }, headers=student_headers)
    assert st_start == 200 and r_start.get('success') is True, f'start exam failed: {st_start} {r_start}'
    attempt_token = r_start.get('data', {}).get('attempt_token')
    assert attempt_token and len(attempt_token) > 10, 'Expected valid attempt_token'
    set_cookie = start_headers.get('Set-Cookie') or start_headers.get('set-cookie', '')
    assert 'aether_exam=' in set_cookie, f'Expected aether_exam cookie in headers: {start_headers}'
    # Extract cookie
    content_cookie = [c for c in set_cookie.split(';') if 'aether_exam=' in c][0].strip()
    print(f'   - /api/student/start: PASS (attempt_token={attempt_token[:8]}..., aether_exam cookie set)')

    # Check iSpring exam package content delivery using content-session cookie
    content_headers = {'Cookie': content_cookie}
    st_content, r_content, _ = http_get('/api/exam/content/index.html', headers=content_headers)
    assert st_content == 200, f'Expected 200 for exam content, got {st_content}'
    assert '<html' in str(r_content).lower(), 'Expected HTML content for iSpring player'
    print('   - /api/exam/content/index.html: PASS (iSpring package served correctly)')

    # 5. Test wrong password
    print('[TEST] 4. Testing wrong password for 2026001...')
    st, r, _ = http_post('/api/auth/student-login', {
        'no_id': '2026001',
        'password': 'wrongpassword',
        'token': 'AETHER1'
    })
    print(f'   Status: {st}, Error: {r.get("error")}')
    assert st == 401, f'Expected 401, got {st}'
    assert r.get('success') is False
    assert r.get('error') == 'Invalid credentials'
    print('   [PASS] Wrong password correctly rejected with 401 and "Invalid credentials"!')

    # 6. Test invalid exam token
    print('[TEST] 5. Testing invalid exam token...')
    st, r, _ = http_post('/api/auth/student-login', {
        'no_id': '2026001',
        'password': 'siswa123',
        'token': 'INVALIDTOKEN'
    })
    print(f'   Status: {st}, Error: {r.get("error")}')
    assert st == 401, f'Expected 401, got {st}'
    assert r.get('success') is False
    assert r.get('error') == 'Invalid exam token'
    print('   [PASS] Invalid token correctly rejected with 401 and "Invalid exam token"!')

    # 7. Test expired exam token
    print('[TEST] 6. Testing expired exam token...')
    conn = sqlite3.connect(DB_PATH)
    cur = conn.cursor()
    cur.execute("""
        INSERT INTO exam_session (tenant_id, exam_id, nama, waktu_mulai, waktu_selesai, token, status)
        VALUES (1, 1, 'Expired Session Test', '2020-01-01 00:00:00', '2020-01-02 00:00:00', 'EXPIRED_TOKEN', 'aktif')
    """)
    expired_sess_id = cur.lastrowid
    cur.execute('INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (?, 1)', (expired_sess_id,))
    conn.commit()
    conn.close()

    try:
        st, r, _ = http_post('/api/auth/student-login', {
            'no_id': '2026001',
            'password': 'siswa123',
            'token': 'EXPIRED_TOKEN'
        })
        print(f'   Status: {st}, Error: {r.get("error")}')
        assert st == 401, f'Expected 401, got {st}'
        assert r.get('success') is False
        assert r.get('error') == 'session has ended'
        print('   [PASS] Expired token correctly rejected with 401 and "session has ended"!')
    finally:
        conn = sqlite3.connect(DB_PATH)
        cur = conn.cursor()
        cur.execute('DELETE FROM exam_session_kelas WHERE session_id = ?', (expired_sess_id,))
        cur.execute('DELETE FROM exam_session WHERE id = ?', (expired_sess_id,))
        conn.commit()
        conn.close()

    # 8. Test supervisor login for lab1 and lab2
    print('[TEST] 7. Testing supervisor login and live monitoring...')
    st1, r1, _ = http_post('/api/auth/supervisor-login', {'username': 'lab1', 'password': 'password123'})
    assert st1 == 200 and r1.get('success') is True, f'lab1 login failed: {st1} {r1}'
    lab1_token = r1.get('data', {}).get('token')
    print('   - Supervisor lab1 login: PASS')

    st2, r2, _ = http_post('/api/auth/supervisor-login', {'username': 'lab2', 'password': 'password123'})
    assert st2 == 200 and r2.get('success') is True, f'lab2 login failed: {st2} {r2}'
    lab2_token = r2.get('data', {}).get('token')
    print('   - Supervisor lab2 login: PASS')

    st_bad, r_bad, _ = http_post('/api/auth/supervisor-login', {'username': 'lab1', 'password': 'wrongpassword'})
    assert st_bad == 401 and r_bad.get('success') is False, f'supervisor bad pass failed: {st_bad} {r_bad}'
    print('   - Supervisor wrong pass rejected: PASS')

    # Supervisor lab1 should see students 1-5, and student 1 should be in progress
    st_room1, r_room1, _ = http_get('/api/supervisor/room-status', headers={'Authorization': f'Bearer {lab1_token}'})
    assert st_room1 == 200 and r_room1.get('success') is True, f'room-status failed: {st_room1} {r_room1}'
    students_r1 = r_room1.get('data', [])
    assert len(students_r1) == 5, f'Expected 5 students in Ruang 1, got {len(students_r1)}'
    assert {s['no_id'] for s in students_r1} == {'2026001', '2026002', '2026003', '2026004', '2026005'}
    s1_status = [s for s in students_r1 if s['no_id'] == '2026001'][0]
    assert s1_status['status'] == 'in_progress', f'Expected student 2026001 status in_progress, got {s1_status["status"]}'
    print(f'   - Supervisor Room 1 monitoring: PASS (5 students listed, 2026001 is {s1_status["status"]})')

    # Supervisor lab2 should see students 6-10
    st_room2, r_room2, _ = http_get('/api/supervisor/room-status', headers={'Authorization': f'Bearer {lab2_token}'})
    assert st_room2 == 200 and r_room2.get('success') is True, f'room-status failed: {st_room2} {r_room2}'
    students_r2 = r_room2.get('data', [])
    assert len(students_r2) == 5, f'Expected 5 students in Ruang 2, got {len(students_r2)}'
    assert {s['no_id'] for s in students_r2} == {'2026006', '2026007', '2026008', '2026009', '2026010'}
    print('   - Supervisor Room 2 monitoring: PASS (5 students listed)')

    # Clean up test session in cek_login so database is pristine
    conn = sqlite3.connect(DB_PATH)
    conn.execute('DELETE FROM cek_login WHERE peserta_id = 1')
    conn.commit()
    conn.close()

    # 9. Test admin login
    print('[TEST] 8. Testing admin login...')
    st_admin, r_admin, _ = http_post('/api/auth/login', {'username': 'admin', 'password': 'admin123'})
    assert st_admin == 200 and r_admin.get('success') is True, f'admin login failed: {st_admin} {r_admin}'
    print('   - Admin login: PASS')

    # 10. Test SPA frontend static serving
    print('[TEST] 9. Testing frontend static serving with and without trailing slash...')
    test_routes = [
        '/',
        '/student/login',
        '/student/login/',
        '/supervisor/login',
        '/supervisor/login/',
        '/admin',
        '/admin/'
    ]
    for page in test_routes:
        st_page, html, _ = http_get(page)
        assert st_page == 200, f'Failed to load {page}: {st_page}'
        assert '<!doctype html>' in str(html).lower(), f'Expected HTML for {page}'
        print(f'   - Route {page}: PASS (200 OK, SPA HTML returned)')

    # 11. Inspect compiled production client-side JS bundle
    print('[TEST] 10. Inspecting compiled production JS bundle for guard logic...')
    nodes_dir = os.path.join(RELEASE_DIR, 'web', 'build', '_app', 'immutable')
    found_raw401 = False
    found_path_guard = False
    for root, dirs, files in os.walk(nodes_dir):
        for f in files:
            if f.endswith('.js'):
                content = open(os.path.join(root, f), 'r', encoding='utf-8', errors='ignore').read()
                if 'raw401' in content:
                    found_raw401 = True
                if 'replace' in content and ('student/login' in content or '/admin' in content):
                    found_path_guard = True

    assert found_raw401, 'Expected raw401 in compiled production JS assets'
    assert found_path_guard, 'Expected path guard in compiled production JS assets'
    print('   [PASS] Compiled JS assets contain raw401 handling and normalized path guard!')

    print('\n======================================================')
    print('ALL VERIFICATION SUITES COMPLETED AND PASSED WITH SUCCESS!')
    print('======================================================')

finally:
    print('[TEST] Terminating server process...')
    proc.terminate()
    try:
        proc.wait(timeout=5)
    except Exception:
        proc.kill()
    print('[TEST] Server process terminated cleanly.')
