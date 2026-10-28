#!/usr/bin/env python3
"""Exercise the real JSONL backend with synthetic Telegram data and isolated state."""
import base64
import json
import os
from pathlib import Path
import queue
import subprocess
import tempfile
import threading

ROOT = Path(__file__).resolve().parent.parent
BACKEND = Path(os.environ.get('TMM_SMOKE_BACKEND', str(ROOT / 'build/telegram-media-downloader-backend')))
FAKE = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
def flag(name, default=None):
    return args[args.index(name)+1] if name in args else default
chat = 123
control_path=pathlib.Path(os.environ['TMM_FAKE_CONTROL'])
controls=json.loads(control_path.read_text()) if control_path.exists() else {}
messages = [
    {'id':3,'file':'lesson.mp4','text':'API Design','date':1793000000,'topic':42},
    {'id':2,'file':'notes.pdf','text':'Workshop Notes','date':1792900000,'topic':42},
    {'id':1,'file':'other.pdf','text':'Unrelated topic','date':1792800000,'topic':43}]
messages[0]['text']=controls.get('caption',messages[0]['text'])
if args[0] == 'version':
    print('Version: 0.20.4')
elif args[:2] == ['chat', 'ls']:
    print(json.dumps([{'id':chat,'type':'group','visible_name':'Synthetic Course','topics':[{'id':42,'title':'Backend'},{'id':43,'title':'Frontend'}]}]))
elif args[:2] == ['chat', 'export']:
    topic=flag('--topic')
    selected=[dict(m,raw={'Media':{'Document':{'ID':9000000000000000000+m['id']+controls.get('version',0),'Size':10,'MimeType':'video/mp4' if m['file'].endswith('mp4') else 'application/pdf','Thumbs':[{'Type':'s','Bytes':os.environ['TMM_FAKE_THUMB']}] if m['file'].endswith('mp4') else []}}}) for m in messages if topic is None or m['topic']==int(topic)]
    if flag('-T') == 'last': selected=selected[:int(flag('-i'))]
    pathlib.Path(flag('-o')).write_text(json.dumps({'id':chat,'messages':selected}))
elif args[0] == 'dl':
    data=json.loads(pathlib.Path(flag('-f')).read_text())
    assert len(data['messages']) == 1, 'download expanded beyond one file'
    assert '--group' not in args, 'album expansion must be disabled'
    message=data['messages'][0]
    with open(os.environ['TMM_FAKE_AUDIT'],'a') as log: log.write(str(message['id'])+'\n')
    print('\x1b[32mtruncated...\x1b[0m 5.0% [0.50 B in 1s; ~ETA: 2s; 1.00 B/s',flush=True)
    print('[1.00 B in 2s; 1.00 B/s',flush=True)
    directory=pathlib.Path(flag('-d')); directory.mkdir(parents=True,exist_ok=True)
    (directory/f"{chat}_{message['id']}_{message['file']}").write_bytes(b'synthetic!')
    if controls.get('block_manifest_on_id')==message['id']:
        manifest=next(pathlib.Path(controls['manifest_root']).rglob('.telegram-media-manager-manifest.json'))
        manifest.rename(manifest.with_name('manifest-smoke-backup.json'));manifest.mkdir()
else:
    sys.exit('unsupported fake command')
'''


def main():
    with tempfile.TemporaryDirectory(prefix='tmm-library-smoke-') as temp:
        root = Path(temp)
        fake = root / 'fake-tdl'
        fake.write_text(FAKE)
        fake.chmod(0o700)
        output = root / 'downloads'
        output.mkdir()
        audit = root / 'transfers.txt'
        environment = dict(os.environ, TDL_PATH=str(fake), TMM_STATE_DIR=str(root / 'state'), TMM_FAKE_AUDIT=str(audit), TMM_FAKE_CONTROL=str(root/'control.json'), TMM_FAKE_THUMB=base64.b64encode((ROOT/'apps/desktop/test/fixtures/video-preview.jpg').read_bytes()).decode())

        def start():
            process = subprocess.Popen([str(BACKEND)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=environment)
            events = queue.Queue()

            def drain():
                for line in process.stdout:
                    events.put(json.loads(line))
            threading.Thread(target=drain, daemon=True).start()
            return process, events

        def send(process, request_id, action, params=None):
            process.stdin.write(json.dumps(dict(version=1, id=request_id, action=action, params=params or {})) + '\n')
            process.stdin.flush()

        def until(events, request_id, name):
            seen = []
            while True:
                event = events.get(timeout=15)
                if event.get('id') != request_id:
                    continue
                assert event['event'] != 'error', event
                seen.append(event)
                if event['event'] == name:
                    return event.get('data', {}), seen
                assert event['event'] != 'job_failed', event

        process, events = start()
        try:
            send(process, 'health', 'health')
            until(events, 'health', 'result')
            send(process, 'chats', 'list_chats')
            chats, _ = until(events, 'chats', 'result')
            chat = chats['chats'][0]
            topic = chat['topics'][0]
            query = dict(chat=chat, topic=topic, limit=1)
            send(process, 'recent', 'list_media', query)
            recent, _ = until(events, 'recent', 'result')
            assert len(recent['items']) == 1 and recent['hasMore']
            assert base64.b64decode(recent['items'][0]['thumbnail']).startswith(b'\xff\xd8')
            assert not audit.exists(), 'browsing thumbnails started a full download'
            send(process, 'older', 'list_media', dict(query, limit=100))
            page, _ = until(events, 'older', 'result')
            assert len(page['items']) == 2 and not page['hasMore']
            selection = dict(topic=topic, fileTypes=['pdf'], snapshotId=page['snapshotId'], keys=['123:2:notes.pdf'])
            send(process, 'selected', 'start_media_download', dict(chat=chat, destinationRoot=str(output), selection=selection))
            completed, seen = until(events, 'selected', 'job_completed')
            files = completed['job']['files']
            assert len(files) == 1 and files[0]['state'] == 'completed'
            assert Path(files[0]['savedPath']).read_bytes() == b'synthetic!'
            progress = [event['data']['progress'] for event in seen if event['event'] == 'download_progress']
            assert len(progress) == 2 and all(p['itemKey'] == '123:2:notes.pdf' for p in progress)
            assert progress[-1]['percent'] == 10, progress
            send(process, 'entire', 'start_media_download', dict(chat=chat, destinationRoot=str(output), selection=dict(topic=topic, fileTypes=['pdf','video'], entireScope=True)))
            completed, _ = until(events, 'entire', 'job_completed')
            assert len(completed['job']['files']) == 2
            assert audit.read_text().splitlines() == ['2', '3'], 'duplicate download or wrong topic'
        finally:
            process.stdin.close()
            process.wait(timeout=15)
        process, events = start()
        try:
            send(process, 'restored', 'list_jobs')
            restored, _ = until(events, 'restored', 'result')
            assert len(restored['jobs']) == 2
            assert all(job['state'] == 'completed' and job['topic']['id'] == 42 for job in restored['jobs'])
            assert all(file['availability']=='available' for job in restored['jobs'] for file in job['files'])
            saved=next(file for job in restored['jobs'] for file in job['files'] if file['item']['originalName']=='lesson.mp4')
            assert saved['item']['thumbnail'], 'thumbnail lost after restart'
            original_path=saved['savedPath']
            (root/'control.json').write_text(json.dumps({'caption':'Edited API Design'}))
            renamed_chat=dict(chat,name='Renamed Synthetic Course')
            send(process,'metadata-edit','start_media_download',dict(chat=renamed_chat,destinationRoot=str(output),selection=dict(topic=topic,fileTypes=['pdf','video'],entireScope=True)))
            kept,_=until(events,'metadata-edit','job_completed')
            kept_video=next(file for file in kept['job']['files'] if file['item']['originalName']=='lesson.mp4')
            assert kept_video['savedPath']==original_path
            assert audit.read_text().splitlines()==['2','3'],'metadata edits downloaded saved media again'
            (root/'control.json').write_text(json.dumps({'caption':'Edited API Design','version':1}))
            send(process,'replacement','start_media_download',dict(chat=renamed_chat,destinationRoot=str(output),selection=dict(topic=topic,fileTypes=['video'],entireScope=True)))
            replaced,_=until(events,'replacement','job_completed')
            assert replaced['job']['files'][0]['savedPath']!=original_path,'replacement overwrote old media'
            assert Path(original_path).read_bytes()==b'synthetic!'
            assert audit.read_text().splitlines()==['2','3','3'],'known same-size replacement was skipped'

            disk_output=root/'storage-case';disk_output.mkdir()
            (root/'control.json').write_text(json.dumps({'block_manifest_on_id':2,'manifest_root':str(disk_output)}))
            send(process,'storage-failure','start_media_download',dict(chat=chat,destinationRoot=str(disk_output),selection=dict(fileTypes=['pdf','video'],entireScope=True)))
            failed,_=until(events,'storage-failure','job_failed')
            job=failed['job']
            assert job['errorCode']=='storage_error' and 'manifest' in job['errorMessage'],(job['errorCode'],job['errorMessage'])
            assert [file['state'] for file in job['files']]==['completed','failed','queued'],job
            assert audit.read_text().splitlines()==['2','3','3','3','2'],'storage failure did not stop later transfers'
            manifest=Path(job['destination'])/'.telegram-media-manager-manifest.json'
            manifest.rmdir();manifest.with_name('manifest-smoke-backup.json').rename(manifest)
            (root/'control.json').write_text('{}')
            send(process,'recover-storage','retry_download',{'jobId':'storage-failure'})
            recovered,_=until(events,'recover-storage','job_completed')
            assert recovered['job']['files'][0]['savedPath']==job['files'][0]['savedPath']
            assert audit.read_text().splitlines()==['2','3','3','3','2','2','1'],'retry downloaded an already saved file'
            Path(original_path).unlink()
            send(process, 'recheck', 'list_jobs')
            checked,_=until(events,'recheck','result')
            assert any(file.get('availability')=='missing' for job in checked['jobs'] for file in job['files'])
        finally:
            process.stdin.close()
            process.wait(timeout=15)
        print('Library protocol smoke passed: topics, thumbnail metadata, exact selection, per-file progress, metadata edits, exact replacements, storage-error stop/retry, restart persistence and missing-file checks.')


if __name__ == '__main__':
    main()
