# Etapa 2 — cliente-servidor

## Preparação

Abra dois terminais PowerShell na raiz do projeto. Compile a versão nova:

```powershell
.\scripts\go.ps1 build -o bin/rc2.exe ./cmd/rc2
New-Item -ItemType Directory -Force work | Out-Null
Set-Content work/exemplo.txt 'Transferencia autenticada por UDP'
```

Se usar Go no PATH, substitua `.\scripts\go.ps1` por `go`.

## 1. Iniciar o servidor

No terminal 1:

```powershell
$env:RC2_PASSWORD = 'senha-de-teste'
.\bin\rc2.exe serve -addr 127.0.0.1:9000 -dir data/server
```

O processo permanece ativo até CTRL+C. A pasta é criada se não existir. A senha é somente um exemplo para teste local.

## 2. Autenticar e listar

No terminal 2:

```powershell
$env:RC2_PASSWORD = 'senha-de-teste'
.\bin\rc2.exe list -addr 127.0.0.1:9000
```

Esperado: JSON com nomes e tamanhos. Uma pasta vazia retorna `[]`.

## 3. Enviar e baixar

```powershell
.\bin\rc2.exe upload -addr 127.0.0.1:9000 -file work/exemplo.txt -name exemplo.txt
Start-Sleep -Seconds 8
.\bin\rc2.exe list -addr 127.0.0.1:9000
.\bin\rc2.exe download -addr 127.0.0.1:9000 -name exemplo.txt -file work/copia.txt
Get-FileHash work/exemplo.txt, work/copia.txt -Algorithm SHA256
```

**Aprovação:** upload e download retornam `success: true`, a listagem inclui `exemplo.txt` e os hashes são iguais. O servidor permanece ativo durante a sequência. Use nomes novos se repetir os testes: os arquivos existentes são preservados.

O upload retorna antes de terminar a espera final do receptor no servidor. Com os parâmetros padrão, aguarde oito segundos antes de outra operação. Um cliente que acessar durante uma transferência pode receber `servidor ocupado`. No download, a espera ocorre no cliente antes de ele retornar.

## 4. Verificar rejeições

```powershell
$env:RC2_PASSWORD = 'senha-errada'
.\bin\rc2.exe list -addr 127.0.0.1:9000
$env:RC2_PASSWORD = 'senha-de-teste'

.\bin\rc2.exe download -addr 127.0.0.1:9000 -name inexistente.bin -file work/ausente.bin
.\bin\rc2.exe upload -addr 127.0.0.1:9000 -file work/exemplo.txt -name exemplo.txt
.\bin\rc2.exe download -addr 127.0.0.1:9000 -name ../fora.txt -file work/fora.txt
```

**Aprovação:** mensagens de senha incorreta, arquivo inexistente, destino existente e nome inválido, respectivamente. Comandos com falha devem retornar código de saída diferente de zero (`$LASTEXITCODE`). Nenhum arquivo fora da pasta deve ser criado ou lido.

## 5. Arquivo grande

Use `work/original.bin`, criado no roteiro anterior, para um novo upload/download. Escolha nomes novos e compare hashes e tamanhos. Esse teste ainda é local; não representa os cenários de RTT/perda do relatório.

## Limites conhecidos

- Uma operação ativa por servidor. Concorrência entre sessões será tratada posteriormente.
- Senha em texto claro; sem criptografia, proteção de replay malicioso ou limitação de tentativas de senha.
- Nomes ASCII simples, sem subpastas. A pasta não deve ser alterada por usuários locais não confiáveis durante operações; a validação por caminho não protege contra trocas locais concorrentes de arquivos/links.
- Listagem em páginas de cinco arquivos, sem snapshot: evite alterar a pasta durante a listagem.
- Cache de até 128 pedidos por dois minutos. Repetições do mesmo pedido recebem resposta anterior e não iniciam outra transferência. Pedidos encerrados são rejeitados; uma nova operação usa novo ID.
- RTO e tentativas ainda são parâmetros locais: use os mesmos valores nos dois lados. Uma falha após a negociação pode aparecer ao cliente como timeout; o servidor registra o erro localmente.

## Testes automatizados

```powershell
.\scripts\go.ps1 test ./... -count=1 -v
.\scripts\go.ps1 vet ./...
```

Não é necessária nenhuma biblioteca adicional.
