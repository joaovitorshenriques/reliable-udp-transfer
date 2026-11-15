"""Decode RC2 UDP payloads, verify CRC and identify repeated DATA on the capture."""
import argparse, collections, csv, hashlib, shutil, subprocess, zlib
from pathlib import Path

def inspect(source, output, tshark):
    command=[tshark,"-r",str(source),"-Y","udp","-T","fields","-e","frame.number","-e","frame.time_relative","-e","ip.src","-e","udp.srcport","-e","ip.dst","-e","udp.dstport","-e","udp.payload"]
    result=subprocess.run(command,check=True,capture_output=True,text=True)
    rows=[]; seen={}; kinds=collections.Counter(); repeats=[]
    names={1:"START",2:"DATA",3:"ACK",4:"FINISH",5:"DONE",6:"FAILURE",7:"REQUEST",8:"RESPONSE"}
    for line in result.stdout.splitlines():
        parts=line.split("\t")
        if len(parts)!=7: continue
        frame,time,src,sport,dst,dport,payload=parts
        b=bytes.fromhex(payload.replace(":",""))
        if b[:4]!=b"RC2U": continue
        if len(b)<28 or b[4]!=1 or b[5] not in names or len(b)!=28+int.from_bytes(b[22:24],"big"): raise ValueError("Invalid RC2 frame "+frame)
        clean=b[:24]+bytes(4)+b[28:]
        if zlib.crc32(clean)!=int.from_bytes(b[24:28],"big"): raise ValueError("CRC mismatch frame "+frame)
        session=b[8:16].hex(); seq=int.from_bytes(b[16:20],"big"); kind=names[b[5]]
        key=(src,sport,dst,dport,session,seq)
        previous=seen.get(key,"") if kind=="DATA" else ""
        if kind=="DATA": seen.setdefault(key,frame)
        row=dict(frame=frame,time_seconds=time,source=src,source_port=sport,destination=dst,destination_port=dport,session=session,kind=kind,sequence=seq,first_data_frame=previous)
        rows.append(row); kinds[kind]+=1
        if previous: repeats.append(row)
    if not rows: raise ValueError("No RC2 packets")
    output.mkdir(parents=True,exist_ok=True)
    with (output/"packets.csv").open("w",newline="",encoding="utf-8") as f:
        w=csv.DictWriter(f,fieldnames=rows[0]); w.writeheader(); w.writerows(rows)
    text=["# RC2 capture inspection", "",f"Source: `{source.name}`; SHA256: `{hashlib.sha256(source.read_bytes()).hexdigest()}`.",f"Validated RC2 packets (length/version/CRC): {len(rows)}.",f"Repeated DATA visible on capture: {len(repeats)}.","", "| Type | Count |", "|---|---:|"]
    text += [f"| {k} | {v} |" for k,v in sorted(kinds.items())]
    text += ["", "## Repeated DATA examples", "", "| First frame | Repeated frame | Session | Sequence |", "|---|---|---|---|"]
    text += [f"| {r['first_data_frame']} | {r['frame']} | {r['session']} | {r['sequence']} |" for r in repeats[:12]]
    text += ["", "Repeated DATA are observations, not an exact count of sender retransmissions: emulator drops precede capture and capture duplication is also possible. ACK sequences can be correlated in packets.csv by session and reverse endpoints. Capture timestamps exclude the delay before the socket send; do not treat DATA/ACK capture deltas as the full virtual RTT."]
    (output/"capture.md").write_text("\n".join(text)+"\n",encoding="utf-8")
    print(f"Validated {len(rows)} packets; repeated DATA: {len(repeats)}")

if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__); p.add_argument("capture",type=Path); p.add_argument("--out",type=Path,required=True)
    p.add_argument("--tshark",default=shutil.which("tshark") or r"C:\Program Files\Wireshark\tshark.exe")
    a=p.parse_args(); inspect(a.capture,a.out,a.tshark)
