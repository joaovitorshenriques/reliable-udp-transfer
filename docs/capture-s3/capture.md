# RC2 capture inspection

Source: `s3-w64.pcapng`; SHA256: `13f28141dc105d43af4c0b656d046b799bef51f39a93cef199febcf8668b3a93`.
Validated RC2 packets (length/version/CRC): 3773.
Repeated DATA visible on capture: 143.

| Type | Count |
|---|---:|
| ACK | 1830 |
| DATA | 1933 |
| DONE | 2 |
| FINISH | 2 |
| REQUEST | 2 |
| RESPONSE | 2 |
| START | 2 |

## Repeated DATA examples

| First frame | Repeated frame | Session | Sequence |
|---|---|---|---|
| 6 | 118 | 5ff4c7fbc5eaed7b | 2 |
| 16 | 119 | 5ff4c7fbc5eaed7b | 12 |
| 31 | 121 | 5ff4c7fbc5eaed7b | 30 |
| 17 | 122 | 5ff4c7fbc5eaed7b | 13 |
| 46 | 126 | 5ff4c7fbc5eaed7b | 46 |
| 41 | 127 | 5ff4c7fbc5eaed7b | 40 |
| 141 | 221 | 5ff4c7fbc5eaed7b | 68 |
| 149 | 222 | 5ff4c7fbc5eaed7b | 76 |
| 167 | 224 | 5ff4c7fbc5eaed7b | 95 |
| 117 | 229 | 5ff4c7fbc5eaed7b | 65 |
| 139 | 231 | 5ff4c7fbc5eaed7b | 66 |
| 46 | 233 | 5ff4c7fbc5eaed7b | 46 |

Repeated DATA are observations, not an exact count of sender retransmissions: emulator drops precede capture and capture duplication is also possible. ACK sequences can be correlated in packets.csv by session and reverse endpoints. Capture timestamps exclude the delay before the socket send; do not treat DATA/ACK capture deltas as the full virtual RTT.
