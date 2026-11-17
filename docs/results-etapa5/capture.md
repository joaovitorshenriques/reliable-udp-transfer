# LAN capture verification

15 sessions matched: START, first DATA (sequence 1), DONE and expected sender. All 194 exported frames passed RC2 header/length/CRC validation. This is a targeted inspection, not a CRC check of all captured DATA.

Five CDN selections: ten probes each (five per replica), successful responses matched by session and reverse endpoints before transfer START; chosen source agrees with minimum eligible median in the JSON report. Probe timestamps are not assumed identical to application RTT measurements.

| Mode | Repetition | Session | Source | First DATA frame |
|---|---|---|---|---|
| desktop | 1 | 8df1fe1dbe00a533 | 192.168.2.128 | 5 |
| desktop | 2 | 3ef2f0099db699f1 | 192.168.2.128 | 716219 |
| desktop | 3 | e3e91fdfeb741e73 | 192.168.2.128 | 1002555 |
| desktop | 4 | 9d515139011eb7fa | 192.168.2.128 | 1290279 |
| desktop | 5 | beb5407123036f7b | 192.168.2.128 | 2007137 |
| notebook | 1 | d0367f999811b413 | 192.168.2.150 | 143163 |
| notebook | 2 | 476603efd3c1383d | 192.168.2.150 | 429883 |
| notebook | 3 | 665ce1712551e991 | 192.168.2.150 | 1145713 |
| notebook | 4 | 900fa9e4b77d6ed5 | 192.168.2.150 | 1433439 |
| notebook | 5 | d0cf7139be47e8e7 | 192.168.2.150 | 1720033 |
| cdn | 1 | fd26e82c85526cdf | 192.168.2.128 | 286715 |
| cdn | 2 | 6a43f47cb5897335 | 192.168.2.128 | 573051 |
| cdn | 3 | c19843e1b242ce07 | 192.168.2.128 | 859388 |
| cdn | 4 | fc3956ad024fb22d | 192.168.2.128 | 1576872 |
| cdn | 5 | 7932115567fe5a19 | 192.168.2.128 | 1863970 |

Control errors:
- Frame 1720028: limite de clientes; tente novamente

The admission failure and later resumed round are retained as experimental limitations. No claim of a continuous campaign or two remote paths is made. Raw capture includes test credentials; only sanitized evidence is published.

## Provenance
- stage5-control.tsv: SHA256 `89dfec4d5a54b2000e89d5e0822fc8f6cd37960db4d0d5e0cf603f69bec019e6`
- stage5-data-first.tsv: SHA256 `4eceeac3caac6e24e096db01612b470c761b12e9b079f85c18b92d35f9ac0cca`
- paired.csv: SHA256 `a6fd67d04edf955d184ba48e7d581ad0437d1159ba320a11292bb27543346f49`
- manifest.json: SHA256 `09146bfc96ac245c6b25894edbdd02f86631b709c88f35802dad2e301856fff2`
