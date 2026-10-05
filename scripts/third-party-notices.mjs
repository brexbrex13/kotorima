import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const root=process.cwd();
const lock=JSON.parse(fs.readFileSync('frontend/package-lock.json','utf8'));
const texts=['Kotorima — Third-party licenses and copyright notices\n\nKotorima is MIT-licensed. DOMPurify is used under Apache-2.0; go-toast is used under MIT. Other components retain their own licenses.\n'];
const missing=[];
const add=(name,version,dir,license='')=>{
 const files=fs.readdirSync(dir).filter(f=>/^(license|licence|copying|notice|copyright)([-._].*)?$/i.test(f)&&fs.statSync(path.join(dir,f)).isFile());
 if(!files.length){if(name==='@wailsio/runtime'){texts.push('\n'+name+' '+version+' (MIT)\n'+fs.readFileSync(path.join(execFileSync(process.env.KOTORIMA_GO??'go',['list','-m','-f','{{.Dir}}','github.com/wailsapp/wails/v3'],{encoding:'utf8'}).trim(),'LICENSE'),'utf8'));return}missing.push(name);return}
 texts.push('\n'+'='.repeat(72)+'\n'+name+' '+version+(license?' ('+license+')':'')+'\n'+'='.repeat(72));
 for(const f of files)texts.push('\n--- '+f+' ---\n'+fs.readFileSync(path.join(dir,f),'utf8'));
};
for(const [dir,p] of Object.entries(lock.packages)){if(!dir||p.dev||p.devOptional)continue;const absolute=path.join(root,'frontend',dir);if(!fs.existsSync(absolute))continue;const info=JSON.parse(fs.readFileSync(path.join(absolute,'package.json'),'utf8'));add(info.name,info.version,absolute,info.license??'')}
const go=process.env.KOTORIMA_GO??'go';
const modules=execFileSync(go,['list','-deps','-f','{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}','.'],{env:{...process.env,GOOS:'windows',GOARCH:'amd64',CGO_ENABLED:'0'},encoding:'utf8'}).trim().split('\n');
for(const entry of new Set(modules)){if(!entry||entry.startsWith('kotorima|'))continue;const [name,version,dir]=entry.split('|');add(name,version,dir)}
const goroot=execFileSync(go,['env','GOROOT'],{encoding:'utf8'}).trim();add('Go standard library','',goroot,'BSD-3-Clause');
fs.writeFileSync('THIRD_PARTY_NOTICES.txt',texts.join('\n'));
if(missing.length){console.error('Missing license files:',missing.join(', '));process.exitCode=1}
