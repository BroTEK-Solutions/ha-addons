package main

import (
	"os/exec"
	"strings"
	"testing"
)

// Exercise the actual embedded renderer with its browser DOM boundary replaced
// by a minimal fixture. Nulls must not be formatted as zero or disconnected.
func TestPageRendersUnknownMeasurements(t *testing.T) {
	script := strings.Split(strings.Split(page, "<script>")[1], "</script>")[0]
	fixture := `
const assert = require('node:assert/strict');
class Element {
 constructor(){this.children=[];this.textContent='';}
 replaceChildren(...children){this.children=children;}
 append(...children){this.children.push(...children);}
 addEventListener(){}
}
const elements=new Map();
global.document={getElementById:id=>{if(!elements.has(id))elements.set(id,new Element());return elements.get(id)},createElement:()=>new Element(),createTextNode:text=>({textContent:text})};
global.fetch=()=>new Promise(()=>{});
` + script + `
const response={agent_responding:true,endpoint:{},options:{},diagnostics:{connected:null,pushes:0,checks:[{type:'HTTP',running:0,executions:null,errors:null}]},checked_at:'2026-01-01T00:00:00Z'};
render(response);
assert.equal(elements.get('cloud').textContent,'Cloud state unknown');
assert.equal(elements.get('totals').children[0].children[0].textContent,'Unknown');
assert.equal(elements.get('totals').children[1].children[0].textContent,'0');
assert.equal(elements.get('checks').children[0].children[1].textContent,'0');
assert.equal(elements.get('checks').children[0].children[2].textContent,'Unknown');
response.diagnostics.connected=false;render(response);
assert.equal(elements.get('cloud').textContent,'Not connected to Grafana Cloud');
response.diagnostics.connected=true;render(response);
assert.equal(elements.get('cloud').textContent,'Connected to Grafana Cloud');
`
	command := exec.Command("node")
	command.Stdin = strings.NewReader(fixture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("renderer: %v\n%s", err, output)
	}
}
