let currentSessionId = null;
let analysisResult = null;
let isRecording = false;

// Initialize sync URLs based on current browser location
function initSyncUrls() {
    const baseUrl = window.location.origin;
    document.getElementById('sync-url-create').textContent = baseUrl + '/sync/create';
    document.getElementById('sync-url-revoke').textContent = baseUrl + '/sync/revoke';
    document.getElementById('sync-url-rotate').textContent = baseUrl + '/sync/rotate';
}

// Run on page load
document.addEventListener('DOMContentLoaded', initSyncUrls);

// Helper to safely create elements with text content
function createElement(tag, className, textContent) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (textContent) el.textContent = textContent;
    return el;
}

// Helper to clear and set text content safely
function setResultText(element, text, className) {
    element.textContent = '';
    const span = createElement('span', className, text);
    element.appendChild(span);
}

// Helper to create an endpoint item safely
function createEndpointItem(method, path, source, bgStyle, borderStyle) {
    const div = document.createElement('div');
    div.className = 'endpoint-item';
    div.style.cssText = bgStyle || '';
    if (borderStyle) div.style.borderLeft = borderStyle;

    const methodSpan = createElement('span', 'method method-' + method.toLowerCase(), method);
    const pathSpan = createElement('span', null, path + ' ');
    pathSpan.style.fontWeight = '500';

    const sourceSmall = document.createElement('small');
    sourceSmall.style.color = '#94a3b8';
    sourceSmall.textContent = '(' + (source || 'recording') + ')';
    pathSpan.appendChild(sourceSmall);

    div.appendChild(methodSpan);
    div.appendChild(pathSpan);
    return div;
}

async function toggleRecording() {
    if (!isRecording) {
        await startRecording();
    } else {
        await stopRecording();
    }
}

async function startRecording() {
    const url = document.getElementById('appUrl').value.trim();
    const resultBox = document.getElementById('discoveryResult');
    const discoverBtn = document.getElementById('discoverBtn');
    const discoverText = document.getElementById('discoverText');

    if (!url) {
        setResultText(resultBox, 'Please enter an application URL', 'result-error');
        return;
    }

    // Update UI - button turns RED and shows "Recording..."
    discoverBtn.classList.remove('btn-primary', 'btn-ready');
    discoverBtn.classList.add('btn-recording');
    discoverText.textContent = '🔴 Recording... (Click to Stop)';
    document.getElementById('appUrl').disabled = true;

    document.getElementById('step1').classList.remove('active');
    document.getElementById('step1').classList.add('done');
    document.getElementById('step2').classList.add('active');
    setResultText(resultBox, 'Starting recording session...', 'result-info');

    try {
        const res = await fetch('/api/v1/sessions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ target_url: url })
        });

        const data = await res.json();
        currentSessionId = data.session_id;
        isRecording = true;

        const proxyUrl = window.location.origin + '/proxy/' + data.session_id + '/';

        // Build result safely using DOM methods
        resultBox.textContent = '';

        resultBox.appendChild(createElement('span', 'result-success', '✓ Recording started!'));
        resultBox.appendChild(document.createTextNode('\n\n'));
        resultBox.appendChild(createElement('span', 'result-info', 'Access your application through this proxy URL:'));
        resultBox.appendChild(document.createTextNode('\n'));

        const link = document.createElement('a');
        link.href = proxyUrl;
        link.target = '_blank';
        link.style.cssText = 'color: #60a5fa; word-break: break-all;';
        link.textContent = proxyUrl;
        resultBox.appendChild(link);

        resultBox.appendChild(document.createTextNode('\n\n'));
        const whatToDo = createElement('span', 'result-info', 'What to do:');
        whatToDo.style.fontWeight = 'bold';
        resultBox.appendChild(whatToDo);
        resultBox.appendChild(document.createTextNode('\n'));
        resultBox.appendChild(document.createTextNode('1. Click the link above to open your app through the proxy\n'));
        resultBox.appendChild(document.createTextNode('2. Login with admin credentials\n'));
        resultBox.appendChild(document.createTextNode('3. Create a new token/API key (CREATE operation)\n'));
        resultBox.appendChild(document.createTextNode('4. Delete/revoke a token if possible (REVOKE operation)\n'));
        resultBox.appendChild(document.createTextNode('5. Rotate/update credentials if available (ROTATE operation)\n'));
        resultBox.appendChild(document.createTextNode('6. Click the RED button above to stop recording'));

    } catch (err) {
        setResultText(resultBox, 'Error: ' + err.message, 'result-error');
        discoverBtn.classList.remove('btn-recording');
        discoverBtn.classList.add('btn-primary');
        discoverText.textContent = '🔍 Discover Application';
        document.getElementById('appUrl').disabled = false;
        isRecording = false;
    }
}

async function stopRecording() {
    if (!currentSessionId) {
        alert('No active recording session');
        return;
    }

    const resultBox = document.getElementById('discoveryResult');
    const discoverBtn = document.getElementById('discoverBtn');
    const discoverText = document.getElementById('discoverText');

    // Show analyzing state
    discoverText.textContent = '⏳ Analyzing...';
    setResultText(resultBox, 'Analyzing recorded traffic for credential endpoints...', 'result-info');

    try {
        const res = await fetch('/api/v1/sessions/' + currentSessionId + '/stop', {
            method: 'POST'
        });

        const data = await res.json();
        analysisResult = data;
        isRecording = false;

        // Button turns GREEN when stopped
        discoverBtn.classList.remove('btn-recording', 'btn-primary');
        discoverBtn.classList.add('btn-ready');
        discoverText.textContent = '✅ Endpoints Discovered';

        document.getElementById('step2').classList.remove('active');
        document.getElementById('step2').classList.add('done');

        // Clear and rebuild result box safely
        resultBox.textContent = '';

        if (data.success) {
            document.getElementById('step3').classList.add('active');

            resultBox.appendChild(createElement('span', 'result-success', '✓ ' + data.message));
            resultBox.appendChild(document.createTextNode('\n\n'));

            // Show API spec info if found
            if (data.spec_found) {
                const specInfo = createElement('span', 'result-info', '🎯 API Spec Found: ' + data.spec_url);
                specInfo.style.cssText = 'color: #10b981; font-weight: bold;';
                resultBox.appendChild(specInfo);
                resultBox.appendChild(document.createTextNode('\n\n'));
            }

            resultBox.appendChild(createElement('span', 'result-info', '🔐 Auth Type: '));
            resultBox.appendChild(createElement('strong', null, data.detected_auth_type || 'unknown'));
            resultBox.appendChild(document.createTextNode('\n'));
            resultBox.appendChild(createElement('span', 'result-info', '📊 Recorded: '));
            resultBox.appendChild(createElement('strong', null, data.total_requests + ' requests'));
            resultBox.appendChild(document.createTextNode('\n\n'));

            // Show detected credential endpoints with emphasis
            const hasEndpoints = data.login_endpoint || data.create_endpoint || data.revoke_endpoint || data.rotate_endpoint;
            if (hasEndpoints) {
                const header = createElement('span', 'result-success', '🎯 Discovered Credential Endpoints:');
                header.style.cssText = 'font-weight: bold; font-size: 1.1em;';
                resultBox.appendChild(header);
                resultBox.appendChild(document.createTextNode('\n'));
            }

            if (data.login_endpoint) {
                resultBox.appendChild(createEndpointItem('LOGIN', data.login_endpoint.path, data.login_endpoint.source, 'background: rgba(59,130,246,0.1);'));
            }
            if (data.create_endpoint) {
                resultBox.appendChild(createEndpointItem('CREATE', data.create_endpoint.path, data.create_endpoint.source, 'background: rgba(16,185,129,0.1);', '3px solid #10b981'));
            }
            if (data.revoke_endpoint) {
                resultBox.appendChild(createEndpointItem('REVOKE', data.revoke_endpoint.path, data.revoke_endpoint.source, 'background: rgba(239,68,68,0.1);', '3px solid #ef4444'));
            }
            if (data.rotate_endpoint) {
                const rotateItem = createEndpointItem('ROTATE', data.rotate_endpoint.path, data.rotate_endpoint.source || 'spec', 'background: rgba(245,158,11,0.1);', '3px solid #f59e0b');
                rotateItem.querySelector('.method').style.background = '#f59e0b';
                resultBox.appendChild(rotateItem);
            }

            // Show spec endpoints if found (collapsible)
            if (data.spec_found && data.spec_endpoints && data.spec_endpoints.length > 0) {
                const details = document.createElement('details');
                details.style.marginTop = '1rem';

                const summary = document.createElement('summary');
                summary.style.cssText = 'cursor: pointer; color: #60a5fa; font-weight: bold;';
                summary.textContent = '📋 All API Endpoints (' + data.spec_endpoints.length + ')';
                details.appendChild(summary);

                const endpointList = document.createElement('div');
                endpointList.style.cssText = 'max-height: 200px; overflow-y: auto; margin-top: 0.5rem;';

                data.spec_endpoints.forEach(ep => {
                    const item = document.createElement('div');
                    item.className = 'endpoint-item';
                    item.style.cssText = 'padding: 0.25rem 0.5rem; margin-bottom: 0.25rem; background: rgba(255,255,255,0.02);';

                    const method = createElement('span', 'method method-' + ep.method.toLowerCase(), ep.method);
                    method.style.cssText = 'font-size: 0.65rem; min-width: 45px;';
                    item.appendChild(method);

                    const pathSpan = document.createElement('span');
                    pathSpan.style.fontSize = '0.75rem';
                    pathSpan.textContent = ep.path;
                    if (ep.description) {
                        const desc = document.createElement('small');
                        desc.style.color = '#64748b';
                        desc.textContent = ' - ' + ep.description.substring(0, 40);
                        pathSpan.appendChild(desc);
                    }
                    item.appendChild(pathSpan);
                    endpointList.appendChild(item);
                });

                details.appendChild(endpointList);
                resultBox.appendChild(details);
            }

            // Action buttons
            if (data.create_endpoint) {
                const buttonDiv = document.createElement('div');
                buttonDiv.style.cssText = 'margin-top: 1rem; display: flex; gap: 0.5rem;';

                const createBtn = createElement('button', 'btn btn-success', '➕ Create Producer');
                createBtn.onclick = createProducerFromAnalysis;
                buttonDiv.appendChild(createBtn);

                const resetBtn = createElement('button', 'btn btn-secondary', '🔄 New Discovery');
                resetBtn.onclick = resetRecording;
                buttonDiv.appendChild(resetBtn);

                resultBox.appendChild(buttonDiv);

                if (!data.revoke_endpoint) {
                    const warning = document.createElement('p');
                    warning.style.cssText = 'color: #f59e0b; font-size: 0.85rem; margin-top: 0.5rem;';
                    warning.textContent = '⚠️ Note: No REVOKE endpoint detected. Producer will support CREATE only.';
                    resultBox.appendChild(warning);
                }
            } else {
                const resetBtn = createElement('button', 'btn btn-secondary', '🔄 Try Again');
                resetBtn.onclick = resetRecording;
                resultBox.appendChild(document.createTextNode('\n'));
                resultBox.appendChild(resetBtn);
            }
        } else {
            resultBox.appendChild(createElement('span', 'result-error', '⚠️ ' + (data.message || 'Could not detect auth patterns')));
            resultBox.appendChild(document.createTextNode('\n\n'));

            if (data.all_endpoints && data.all_endpoints.length > 0) {
                resultBox.appendChild(createElement('span', 'result-info', 'Captured '));
                resultBox.appendChild(createElement('strong', null, data.all_endpoints.length + ' endpoints'));
                resultBox.appendChild(document.createTextNode(':\n'));

                data.all_endpoints.slice(0, 10).forEach(ep => {
                    resultBox.appendChild(document.createTextNode('• ' + ep.method + ' ' + ep.path + '\n'));
                });
                if (data.all_endpoints.length > 10) {
                    resultBox.appendChild(document.createTextNode('... and ' + (data.all_endpoints.length - 10) + ' more\n'));
                }
            }

            const resetBtn = createElement('button', 'btn btn-secondary', '🔄 Try Again');
            resetBtn.onclick = resetRecording;
            resultBox.appendChild(document.createTextNode('\n'));
            resultBox.appendChild(resetBtn);
        }
    } catch (err) {
        setResultText(resultBox, 'Error: ' + err.message, 'result-error');
        discoverBtn.classList.remove('btn-recording');
        discoverBtn.classList.add('btn-primary');
        discoverText.textContent = '🔍 Discover Application';
        isRecording = false;
    } finally {
        document.getElementById('appUrl').disabled = false;
    }
}

async function createProducerFromAnalysis() {
    if (!analysisResult) {
        alert('No analysis result available');
        return;
    }

    const resultBox = document.getElementById('discoveryResult');
    setResultText(resultBox, 'Creating producer...', 'result-info');

    // Build producer config from analysis
    const producer = {
        name: analysisResult.target_url.replace(/https?:\/\//, '').split('/')[0] + ' Credentials',
        type: 'rest-api',
        description: 'Auto-discovered from ' + analysisResult.target_url,
        target_url: analysisResult.target_url,
        config: {
            auth_type: analysisResult.detected_auth_type || 'bearer',
            auth_header: 'Authorization'
        }
    };

    // CREATE endpoint is required
    if (analysisResult.create_endpoint) {
        producer.config.create_endpoint = {
            path: analysisResult.create_endpoint.path,
            method: analysisResult.create_endpoint.method,
            body_template: analysisResult.create_endpoint.body_template || ''
        };
    } else {
        alert('CREATE endpoint is required to create a producer');
        setResultText(resultBox, 'Cannot create producer without CREATE endpoint', 'result-error');
        return;
    }

    // REVOKE endpoint is optional
    if (analysisResult.revoke_endpoint) {
        producer.config.revoke_endpoint = {
            path: analysisResult.revoke_endpoint.path,
            method: analysisResult.revoke_endpoint.method
        };
    }

    // ROTATE endpoint is optional
    if (analysisResult.rotate_endpoint) {
        producer.config.rotate_endpoint = {
            path: analysisResult.rotate_endpoint.path,
            method: analysisResult.rotate_endpoint.method
        };
    }

    try {
        const res = await fetch('/api/v1/producers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(producer)
        });

        if (res.ok) {
            const created = await res.json();
            document.getElementById('step3').classList.remove('active');
            document.getElementById('step3').classList.add('done');

            resultBox.textContent = '';
            resultBox.appendChild(createElement('span', 'result-success', '✓ Producer created: ' + created.name));
            resultBox.appendChild(document.createTextNode('\n\n'));
            resultBox.appendChild(createElement('span', 'result-info', 'ID: ' + created.id));
            resultBox.appendChild(document.createTextNode('\n'));
            resultBox.appendChild(createElement('span', 'result-info', 'Click "Deploy to Akeyless" to create a dynamic secret.'));

            loadProducers();
            setTimeout(resetRecording, 3000);
        } else {
            const err = await res.text();
            setResultText(resultBox, 'Failed to create producer: ' + err, 'result-error');
        }
    } catch (err) {
        setResultText(resultBox, 'Error: ' + err.message, 'result-error');
    }
}

function resetRecording() {
    currentSessionId = null;
    analysisResult = null;
    isRecording = false;

    // Reset steps
    ['step1', 'step2', 'step3'].forEach(s => {
        document.getElementById(s).classList.remove('done', 'active');
    });
    document.getElementById('step1').classList.add('active');

    // Reset button to original state
    const discoverBtn = document.getElementById('discoverBtn');
    const discoverText = document.getElementById('discoverText');
    discoverBtn.classList.remove('btn-recording', 'btn-ready');
    discoverBtn.classList.add('btn-primary');
    discoverText.textContent = '🔍 Discover Application';
    discoverBtn.disabled = false;

    // Re-enable URL input
    document.getElementById('appUrl').disabled = false;
    document.getElementById('appUrl').value = '';

    // Reset result box safely
    const resultBox = document.getElementById('discoveryResult');
    resultBox.textContent = '';
    resultBox.appendChild(createElement('span', 'result-info', 'Enter your application URL and click "Discover Application".'));
    resultBox.appendChild(document.createTextNode('\n\n'));
    resultBox.appendChild(createElement('span', 'result-info', 'How it works:'));
    resultBox.appendChild(document.createTextNode('\n1. Enter your app URL and click Discover\n'));
    resultBox.appendChild(document.createTextNode('2. Button turns RED - recording started\n'));
    resultBox.appendChild(document.createTextNode('3. Access app via proxy link\n'));
    resultBox.appendChild(document.createTextNode('4. Perform create/revoke/rotate operations\n'));
    resultBox.appendChild(document.createTextNode('5. Click RED button to stop\n'));
    resultBox.appendChild(document.createTextNode('6. Button turns GREEN - endpoints discovered!'));
}

async function loadProducers() {
    try {
        const res = await fetch('/api/v1/producers');
        const producers = await res.json();
        const container = document.getElementById('producers');
        document.getElementById('producerCount').textContent = producers.length;

        container.textContent = '';

        if (!producers || producers.length === 0) {
            const p = document.createElement('p');
            p.style.color = '#64748b';
            p.textContent = 'No producers configured yet.';
            container.appendChild(p);
            return;
        }

        producers.forEach(p => {
            const card = document.createElement('div');
            card.className = 'producer-card';

            const header = document.createElement('div');
            header.className = 'producer-header';
            header.appendChild(createElement('span', 'producer-name', p.name));
            header.appendChild(createElement('span', 'producer-type', p.type));
            card.appendChild(header);

            card.appendChild(createElement('p', 'producer-desc', p.description || 'No description'));
            card.appendChild(createElement('p', 'producer-url', p.target_url || ''));

            const actions = document.createElement('div');
            actions.className = 'producer-actions';

            const deployBtn = createElement('button', 'btn btn-success btn-sm', 'Deploy to Akeyless');
            deployBtn.onclick = () => deployToAkeyless(p.id);
            actions.appendChild(deployBtn);

            const testBtn = createElement('button', 'btn btn-secondary btn-sm', 'Test');
            testBtn.onclick = () => testProducer(p.id);
            actions.appendChild(testBtn);

            const deleteBtn = createElement('button', 'btn btn-danger btn-sm', 'Delete');
            deleteBtn.onclick = () => deleteProducer(p.id);
            actions.appendChild(deleteBtn);

            card.appendChild(actions);
            container.appendChild(card);
        });
    } catch (err) {
        console.error('Failed to load producers:', err);
    }
}

async function testProducer(id) {
    try {
        const res = await fetch('/api/v1/producers/' + id + '/test', { method: 'POST' });
        const result = await res.json();
        alert(result.message || 'Test completed');
    } catch (err) {
        alert('Test failed: ' + err.message);
    }
}

async function deleteProducer(id) {
    if (!confirm('Delete this producer?')) return;
    try {
        await fetch('/api/v1/producers/' + id, { method: 'DELETE' });
        loadProducers();
    } catch (err) {
        alert('Delete failed: ' + err.message);
    }
}

async function deployToAkeyless(id) {
    const secretPath = prompt('Enter secret path in Akeyless:', '/Dynamic/' + id);
    if (!secretPath) return;

    try {
        const res = await fetch('/api/v1/producers/' + id + '/deploy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ secret_path: secretPath, ttl: '1h' })
        });
        const text = await res.text();
        if (res.ok) {
            const data = JSON.parse(text);
            alert('Deployed successfully!\n\n' + data.message + '\n\nUsage: ' + data.usage);
        } else {
            alert('Failed: ' + text);
        }
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

// Load on page load
document.addEventListener('DOMContentLoaded', loadProducers);
