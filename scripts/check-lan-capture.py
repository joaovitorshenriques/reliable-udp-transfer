"""Cross-check exported RC2 control and first-DATA frames against LAN results.
Inputs are TShark TSV exports: frame, interface, epoch, src, sport, dst, dport, udp.payload.
Exports contain authentication payloads: keep them in ignored work/, never publish them.
"""
import argparse, collections, csv, hashlib, json, zlib
from pathlib import Path

def read(path):
 b=path.read_bytes()
 return b.decode("utf-16" if b[:2] in (b"\xff\xfe",b"\xfe\xff") else "utf-8-sig")

def frames(path):
 result=[]
 for line in read(path).splitlines():
  fields=line.split("\t")
  if len(fields)!=8: raise ValueError("Unexpected TSV row")
  frame,interface,epoch,src,sport,dst,dport,payload=fields
  b=bytes.fromhex(payload.replace(":",""))
  assert b[:5]==b"RC2U\x01" and len(b)==28+int.from_bytes(b[22:24],"big")
  assert zlib.crc32(b[:24]+bytes(4)+b[28:])==int.from_bytes(b[24:28],"big")
  result.append(dict(frame=int(frame),interface=interface,time=float(epoch),src=src,sport=sport,dst=dst,dport=dport,session=b[8:16].hex(),kind=b[5],seq=int.from_bytes(b[16:20],"big"),body=json.loads(b[28:]) if b[5] in (1,7,8) else {}))
 return sorted(result,key=lambda x:x["time"])

def main(a):
 controls=frames(a.control);data=frames(a.data)
 pairs=list(csv.DictReader(a.paired.open(encoding="utf-8-sig")))
 assert len(pairs)==15
 evidence=[]
 for r in pairs:
  sid=r["session"];start=[f for f in controls if f['session']==sid and f['kind']==1]
  done=[f for f in controls if f['session']==sid and f['kind']==5]
  first=[f for f in data if f['session']==sid and f['kind']==2 and f['seq']==1]
  assert start and done and first, "Missing transfer evidence: "+sid
  f=first[0];expected="192.168.2.128" if r['server']=="desktop" else "192.168.2.150"
  assert f['src']==expected and f['sport']=="9000"
  assert all(x['src']==f['src'] and x['dst']==f['dst'] and x['dport']==f['dport'] for x in start)
  assert any(x['src']==f['dst'] and x['sport']==f['dport'] and x['dst']==f['src'] for x in done)
  evidence.append(dict(mode=r['mode'],repetition=r['repetition'],session=sid,server=f['src'],interface=f['interface'],start_frame=start[0]['frame'],first_data_frame=f['frame'],done_frame=done[0]['frame']))
  if r['mode']=='cdn':
   probes=[x for x in controls if x['kind']==7 and x['body'].get('operation')=='probe' and x['time']<start[0]['time']][-10:]
   assert len(probes)==10 and collections.Counter(x['dst'] for x in probes)=={'192.168.2.128':5,'192.168.2.150':5}
   for probe in probes:
    replies=[x for x in controls if x['kind']==8 and x['session']==probe['session'] and x['src']==probe['dst'] and x['dst']==probe['src'] and x['sport']==probe['dport'] and x['dport']==probe['sport'] and probe['frame']<x['time']<start[0]['time']]
    assert replies and all(not x['body'].get('error') for x in replies)
   selection=json.loads(read(a.campaign/f"cdn-{r['repetition']}-selection.json"))
   eligible=[x for x in selection['results'] if x['eligible']]
   assert selection['selected']==min(eligible,key=lambda x:x['median_ms'])['address']==expected+':9000'
 a.out.mkdir(parents=True,exist_ok=True)
 with (a.out/'capture-evidence.csv').open('w',newline='',encoding='utf-8') as handle:
  writer=csv.DictWriter(handle,fieldnames=evidence[0]);writer.writeheader();writer.writerows(evidence)
 errors=[x for x in controls if x['kind']==8 and x['body'].get('error')]
 text=['# LAN capture verification','',f'15 sessions matched: START, first DATA (sequence 1), DONE and expected sender. All {len(controls)+len(data)} exported frames passed RC2 header/length/CRC validation. This is a targeted inspection, not a CRC check of all captured DATA.', '', 'Five CDN selections: ten probes each (five per replica), successful responses matched by session and reverse endpoints before transfer START; chosen source agrees with minimum eligible median in the JSON report. Probe timestamps are not assumed identical to application RTT measurements.', '', '| Mode | Repetition | Session | Source | First DATA frame |','|---|---|---|---|---|']
 text += [f"| {r['mode']} | {r['repetition']} | {r['session']} | {r['server']} | {r['first_data_frame']} |" for r in evidence]
 text += ['', 'Control errors:']+[f"- Frame {x['frame']}: {x['body']['error']}" for x in errors]
 text += ['', 'The admission failure and later resumed round are retained as experimental limitations. No claim of a continuous campaign or two remote paths is made. Raw capture includes test credentials; only sanitized evidence is published.', '', '## Provenance']
 for path in [a.control,a.data,a.paired,a.campaign/'manifest.json']:
  text.append(f"- {path.name}: SHA256 `{hashlib.sha256(path.read_bytes()).hexdigest()}`")
 (a.out/'capture.md').write_text('\n'.join(text)+'\n',encoding='utf-8')
 print(f'PASS: {len(evidence)} transfers, 50 authenticated probes and five matching selections; {len(errors)} recorded control error(s).')

if __name__=='__main__':
 p=argparse.ArgumentParser(description=__doc__)
 for name in ['control','data','paired','campaign','out']:p.add_argument('--'+name,required=True,type=Path)
 main(p.parse_args())
