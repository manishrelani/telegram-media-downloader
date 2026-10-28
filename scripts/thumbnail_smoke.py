#!/usr/bin/env python3
"""Exercise real backend streaming with an isolated, gated thumbnail-only producer."""
import base64
import json
import os
from pathlib import Path
import queue
import subprocess
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parent.parent
BACKEND = Path(os.environ.get('TMM_SMOKE_BACKEND', ROOT / 'build/telegram-media-downloader-backend'))
FAKE = r'''#!/usr/bin/env python3
import json,os,pathlib,sys,time
args=sys.argv[1:]
def flag(name):return args[args.index(name)+1]
def frame(key):print(json.dumps({'protocol':'tmm-previews-v3','key':key,'image':os.environ['TMM_SHARP_THUMB']}),flush=True)
if args[:2]==['chat','export']:
 messages=[]
 for mid in [7,8,9]:
  raw={'Media':{'Document':{'ID':9223372036854775000+mid,'AccessHash':8223372036854775000,'FileReference':'AQI=','DCID':2,'Thumbs':[{'Type':'s','W':40,'H':22,'Size':600,'Bytes':os.environ['TMM_TINY_THUMB']},{'Type':'m','W':320,'H':180,'Size':10000}]}}}
  messages.append({'id':mid,'file':f'fixture-{mid}.mp4','raw':raw})
 pathlib.Path(flag('-o')).write_text(json.dumps({'id':123,'messages':messages}))
elif args==['tmm-previews-v3','--identity']:print('tmm-previews-v3')
elif args[0]=='tmm-previews-v3':
 requests=json.loads(pathlib.Path(flag('--input')).read_text());keys=[r['key'] for r in requests]
 assert all(r['reference']['thumbSize']=='m' and r['reference']['id']>9223372036854775000 for r in requests)
 root=pathlib.Path(os.environ['TMM_GATE_DIR'])
 with open(root/'audit','a') as file:file.write(json.dumps(keys)+'\n')
 if keys==['123:7:fixture-7.mp4','123:8:fixture-8.mp4']:
  print('unrelated output',flush=True);frame('unrequested');frame(keys[0]);frame(keys[0])
  (root/'blocked').touch()
  while not (root/'release').exists():time.sleep(.01)
  frame(keys[1])
 elif keys==['123:9:fixture-9.mp4']:
  (root/'cancel-started').touch()
  while not (root/'resume').exists():time.sleep(.01)
  frame(keys[0])
 else:raise SystemExit('unexpected/offscreen thumbnail request')
else:raise SystemExit('Unexpected command: browsing must never invoke a video transfer')
'''

def main():
 with tempfile.TemporaryDirectory(prefix='tmm-preview-smoke-') as temp:
  root=Path(temp);fake=root/'tdl';fake.write_text(FAKE);fake.chmod(0o700)
  tiny=base64.b64encode((ROOT/'apps/desktop/test/fixtures/low-resolution-preview.jpg').read_bytes()).decode()
  sharp=base64.b64encode((ROOT/'apps/desktop/test/fixtures/video-preview.jpg').read_bytes()).decode()
  env=dict(os.environ,TDL_PATH=str(fake),TMM_STATE_DIR=str(root/'state'),TMM_TINY_THUMB=tiny,TMM_SHARP_THUMB=sharp,TMM_GATE_DIR=str(root))
  process=subprocess.Popen([str(BACKEND)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,env=env)
  events=queue.Queue()
  def read():
   for line in process.stdout:events.put(json.loads(line))
  threading.Thread(target=read,daemon=True).start()
  threading.Thread(target=lambda:list(process.stderr),daemon=True).start()
  def send(request,action,params):
   process.stdin.write(json.dumps({'version':1,'id':request,'action':action,'params':params})+'\n');process.stdin.flush()
  def until(request,event):
   while True:
    message=events.get(timeout=15)
    assert message['event']!='error',message
    if message['id']==request and message['event']==event:return message.get('data',{})
  def wait_file(name):
   deadline=time.monotonic()+10
   while not (root/name).exists():
    assert time.monotonic()<deadline,f'producer did not reach {name}'
    time.sleep(.01)
  keys=[f'123:{i}:fixture-{i}.mp4' for i in [7,8,9]]
  try:
   send('initial','list_media',{'chat':{'id':123,'name':'Fixture','type':'channel'},'limit':3});page=until('initial','result')
   assert all(item['thumbnail']==tiny and 'preview' not in item for item in page['items'])
   assert not (root/'audit').exists(),'list_media fetched thumbnails automatically'
   send('visible','request_thumbnails',{'snapshotId':page['snapshotId'],'keys':keys[:2]})
   first=until('visible','media_thumbnails');assert first['thumbnails']=={keys[0]:sharp}
   wait_file('blocked');assert not (root/'release').exists(),'producer was not blocked'
   (root/'release').touch()
   second=until('visible','media_thumbnails');assert second['thumbnails']=={keys[1]:sharp},'duplicate or unrequested image escaped filtering'
   assert not until('visible','result')['busy']
   send('cached','request_thumbnails',{'snapshotId':page['snapshotId'],'keys':[keys[0]]})
   assert until('cached','media_thumbnails')['thumbnails']=={keys[0]:sharp}
   until('cached','result')
   assert len((root/'audit').read_text().splitlines())==1,'cached image fetched again'
   send('leaving','request_thumbnails',{'snapshotId':page['snapshotId'],'keys':[keys[2]]});wait_file('cancel-started')
   send('cancel','cancel_media_request',{'requestId':'leaving'})
   assert until('leaving','result')['cancelled']
   (root/'resume').touch()
   send('returning','request_thumbnails',{'snapshotId':page['snapshotId'],'keys':[keys[2]]})
   assert until('returning','media_thumbnails')['thumbnails']=={keys[2]:sharp}
   assert not until('returning','result')['busy'],'cancellation did not release CLI slot'
   assert [json.loads(line) for line in (root/'audit').read_text().splitlines()]==[keys[:2],[keys[2]],[keys[2]]]
  finally:
   (root/'release').touch();(root/'resume').touch()
   process.stdin.close();process.wait(timeout=20)
  assert process.returncode==0
  print('Thumbnail smoke passed: per-image streaming before sibling completion, visible keys, exact IDs, cache, cancellation, and zero video transfers.')

if __name__=='__main__':main()
