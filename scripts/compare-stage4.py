"""Validate and compare three complete 80 MiB campaigns (windows 16,64,256)."""
import argparse, csv, importlib.util, json
from pathlib import Path
spec=importlib.util.spec_from_file_location("plot",Path(__file__).with_name("plot-stage4.py"))
module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)

def compare(sources,output):
    manifests=[json.loads((s/"manifest.json").read_text(encoding="utf-8-sig")) for s in sources]
    baseline=manifests[0]; windows=[]
    for m in manifests:
        for key in ["MiB","SHA256","OS","Arch","GoVersion","Repetitions","Config"]:
            if m[key]!=baseline[key]: raise ValueError("Incompatible campaign: "+key)
        if m["MiB"]<80 or m["Repetitions"]!=5: raise ValueError("Require >=80 MiB and five repeats")
        groups=sorted(m["Groups"],key=lambda g:g["Name"])
        if [g["Name"] for g in groups]!=["S1","S2","S3","S4"]: raise ValueError("Require all four scenarios")
        w={g["Window"] for g in groups}
        if len(w)!=1: raise ValueError("Mixed windows")
        windows.append(w.pop())
        for g,b in zip(groups,sorted(baseline["Groups"],key=lambda g:g["Name"])):
            if {k:v for k,v in g.items() if k!="Window"}!={k:v for k,v in b.items() if k!="Window"}: raise ValueError("Incompatible scenario")
    if sorted(windows)!=[16,64,256]: raise ValueError("Require distinct windows 16,64,256")
    rows=[]
    for source,window in zip(sources,windows):
        target=output/f"w{window}"; module.plot(source,target)
        report_path=target/"analysis.md"
        report_text=report_path.read_text(encoding="utf-8").replace("Ainda faltam variação de janela e capturas Wireshark; mini-CDN é outra etapa.", "Esta campanha integra a comparação entre janelas; consulte comparison.md no diretório superior. Captura S3 documentada separadamente; mini-CDN é outra etapa.")
        report_path.write_text(report_text,encoding="utf-8")
        with (target/"summary.csv").open(encoding="utf-8") as f:
            rows += [dict(window=window,**r) for r in csv.DictReader(f)]
    with (output/"comparison.csv").open("w",newline="",encoding="utf-8") as f:
        writer=csv.DictWriter(f,fieldnames=rows[0]); writer.writeheader(); writer.writerows(rows)
    for operation in ["upload","download"]:
        fig,axes=module.plt.subplots(2,2,figsize=(10,7),layout="constrained")
        for ax,scenario in zip(axes.flat,["S1","S2","S3","S4"]):
            selected=sorted([r for r in rows if r["scenario"]==scenario and r["operation"]==operation],key=lambda r:r["window"])
            ax.errorbar([0,1,2],[float(r["goodput_mbps_mean"]) for r in selected],yerr=[float(r["goodput_mbps_ci95_halfwidth"]) for r in selected],fmt="o-",capsize=4)
            ax.set_xticks([0,1,2],[16,64,256]); ax.set_title(scenario); ax.set_xlabel("Window (packets)"); ax.set_ylabel("Goodput (Mbit/s)"); ax.set_ylim(bottom=0); ax.grid(False)
        fig.suptitle(operation+" - sender mean and 95% CI (n=5)")
        for ext in ["png","pdf"]: fig.savefig(output/f"comparison-{operation}.{ext}",dpi=200)
        module.plt.close(fig)
    report=["# Window comparison", "", "Five repetitions per scenario/window; sender metrics, Student t intervals. These intervals do not establish significance or remove scheduling/machine effects. Confirm same machine and source revision manually; manifests record OS/Go but do not identify hardware or commit.", "", "| Scenario | Operation | Window | Goodput Mbit/s | CI95 halfwidth |", "|---|---|---:|---:|---:|"]
    report += [f"| {r['scenario']} | {r['operation']} | {r['window']} | {float(r['goodput_mbps_mean']):.4f} | {float(r['goodput_mbps_ci95_halfwidth']):.4f} |" for r in sorted(rows,key=lambda r:(r['scenario'],r['operation'],r['window']))]
    report += ["", "![Upload](comparison-upload.png)", "", "![Download](comparison-download.png)"]
    (output/"comparison.md").write_text("\n".join(report)+"\n",encoding="utf-8")

if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__); p.add_argument("campaigns",nargs=3,type=Path); p.add_argument("--out",required=True,type=Path)
    a=p.parse_args(); compare(a.campaigns,a.out)
