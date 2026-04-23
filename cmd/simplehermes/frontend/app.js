const state = {
  current: null,
  shortcuts: [],
  pollTimer: null,
  commandChain: Promise.resolve(),
  pendingFocus: null,
  spacePTTActive: false,
  lastAnnouncementText: "",
  lastAnnouncementAt: 0,
  wheelDeltaAccumulator: 0,
  wheelFlushTimer: null,
  audioContext: null,
  rxSocket: null,
  rxNode: null,
  rxBuffers: [],
  rxBufferOffset: 0,
  txSocket: null,
  txNode: null,
  txMonitor: null,
  micStream: null,
  micSource: null,
  lastMicError: "",
  debugOpen: false,
  lastDebugFocusedElement: null,
  debugEvents: [],
  diagnostics: null,
  frontendDiagnostics: {
    audioContextState: "not-created",
    rxSocketState: "closed",
    txSocketState: "closed",
    rxFramesReceived: 0,
    rxSamplesReceived: 0,
    rxPlaybackCallbacks: 0,
    rxUnderruns: 0,
    txFramesSent: 0,
    txSamplesSent: 0,
    micState: "not-requested",
    lastAudioError: "",
  },
  settingsOpen: false,
  lastFocusedElement: null,
  delayedAnnouncementTimer: null,
  delayedAnnouncement: null,
};

const elements = {};

document.addEventListener("DOMContentLoaded", () => {
  bindElements();
  bindActions();
  refreshState();
  state.pollTimer = window.setInterval(refreshState, 1500);
});

function bindElements() {
  elements.main = document.getElementById("main");
  elements.mode = document.getElementById("app-mode");
  elements.remote = document.getElementById("app-remote");
  elements.transport = document.getElementById("app-transport");
  elements.appDevice = document.getElementById("app-device");

  elements.connectionState = document.getElementById("connection-state");
  elements.stationStatus = document.getElementById("station-status");
  elements.liveStatus = document.getElementById("live-status");
  elements.currentFrequency = document.getElementById("current-frequency");
  elements.currentBand = document.getElementById("current-band");
  elements.currentMode = document.getElementById("current-mode");
  elements.currentStep = document.getElementById("current-step");
  elements.currentPower = document.getElementById("current-power");
  elements.currentRadioState = document.getElementById("current-radio-state");
  elements.hardwareSummary = document.getElementById("hardware-summary");

  elements.bandButtons = document.getElementById("band-buttons");
  elements.modeButtons = document.getElementById("mode-buttons");
  elements.deviceList = document.getElementById("device-list");
  elements.shortcuts = document.getElementById("shortcuts");
  elements.settingsModal = document.getElementById("settings-modal");
  elements.settingsDialog = document.getElementById("settings-panel");
  elements.openSettingsButton = document.getElementById("open-settings");
  elements.closeSettingsButton = document.getElementById("close-settings");
  elements.debugModal = document.getElementById("debug-modal");
  elements.debugDialog = document.getElementById("debug-panel");
  elements.openDebugButton = document.getElementById("open-debug");
  elements.closeDebugButton = document.getElementById("close-debug");
  elements.copyDebugButton = document.getElementById("copy-debug");
  elements.clearDebugButton = document.getElementById("clear-debug");
  elements.debugRadio = document.getElementById("debug-radio");
  elements.debugAudio = document.getElementById("debug-audio");
  elements.debugLog = document.getElementById("debug-log");

  elements.frequencyInput = document.getElementById("frequency-input");
  elements.stepSelect = document.getElementById("step-select");
  elements.powerSelect = document.getElementById("power-select");

  elements.rxToggle = document.getElementById("rx-toggle");
  elements.txToggle = document.getElementById("tx-toggle");
  elements.pttToggle = document.getElementById("ptt-toggle");
  elements.audioStart = document.getElementById("audio-start");

  elements.radioForm = document.getElementById("radio-form");
  elements.settingsForm = document.getElementById("settings-form");

  elements.settingsMode = document.getElementById("settings-mode");
  elements.settingsListen = document.getElementById("settings-listen");
  elements.settingsRemote = document.getElementById("settings-remote");
  elements.settingsAccessibility = document.getElementById("settings-accessibility");
}

function bindActions() {
  elements.openDebugButton.addEventListener("click", () => {
    openDebug();
  });

  elements.closeDebugButton.addEventListener("click", () => {
    closeDebug();
  });

  elements.copyDebugButton.addEventListener("click", () => {
    copyDebugReport();
  });

  elements.clearDebugButton.addEventListener("click", () => {
    state.debugEvents = [];
    renderDebug();
  });

  elements.debugModal.addEventListener("click", (event) => {
    if (event.target instanceof HTMLElement && event.target.dataset.closeDebug === "true") {
      closeDebug();
    }
  });

  elements.openSettingsButton.addEventListener("click", () => {
    openSettings();
  });

  elements.closeSettingsButton.addEventListener("click", () => {
    closeSettings();
  });

  elements.settingsModal.addEventListener("click", (event) => {
    if (event.target instanceof HTMLElement && event.target.dataset.closeSettings === "true") {
      closeSettings();
    }
  });

  document.getElementById("discover-button").addEventListener("click", () => {
    sendCommand({ type: "discover" }, () => "Discovery requested.");
  });

  document.getElementById("disconnect-button").addEventListener("click", () => {
    sendCommand({ type: "disconnect" }, () => "Disconnected.");
  });

  elements.radioForm.addEventListener("submit", async (event) => {
    event.preventDefault();

    await sendCommand({ type: "setStep", stepHz: Number(elements.stepSelect.value) });
    await sendCommand({ type: "setPower", powerPercent: Number(elements.powerSelect.value) });
    await sendCommand(
      { type: "setFrequency", frequencyMHz: elements.frequencyInput.value },
      (nextState) => frequencyAnnouncement(nextState.radio)
    );

    if (state.current) {
      syncFrequencyInput(state.current.radio, true);
    }
  });

  elements.frequencyInput.addEventListener("focus", () => {
    elements.frequencyInput.select();
  });

  elements.frequencyInput.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    event.preventDefault();
    if (state.current) {
      syncFrequencyInput(state.current.radio, true);
      elements.frequencyInput.select();
    }
  });

  document.querySelectorAll("[data-nudge]").forEach((button) => {
    button.addEventListener("click", () => {
      sendCommand(
        { type: "nudgeFrequency", steps: Number(button.dataset.nudge) },
        (nextState) => deferredFrequencyAnnouncement(nextState.radio)
      );
    });
  });

  elements.rxToggle.addEventListener("click", () => {
    if (!state.current) return;
    sendCommand(
      { type: "setRX", enabled: !state.current.radio.rxEnabled },
      (nextState) => `Receive ${nextState.radio.rxEnabled ? "on" : "off"}.`
    );
  });

  elements.txToggle.addEventListener("click", () => {
    if (!state.current) return;
    sendCommand(
      { type: "setTX", enabled: !state.current.radio.txEnabled },
      (nextState) => `Transmit ${nextState.radio.txEnabled ? "armed" : "disarmed"}.`
    );
  });

  elements.pttToggle.addEventListener("click", () => {
    if (!state.current) return;
    sendCommand(
      { type: "setPTT", enabled: !state.current.radio.ptt },
      (nextState) => pttAnnouncement(nextState.radio.ptt)
    );
  });

  elements.audioStart.addEventListener("click", () => {
    startAudioFromUserGesture();
  });

  elements.settingsForm.addEventListener("submit", async (event) => {
    event.preventDefault();

    const payload = {
      mode: elements.settingsMode.value,
      listenAddress: elements.settingsListen.value,
      remoteBaseUrl: elements.settingsRemote.value,
      accessibilityMode: elements.settingsAccessibility.checked,
    };

    const response = await fetch("/api/settings", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    const nextState = await response.json();
    applyState(nextState);
    announce("Settings saved.");
  });

  window.addEventListener("keydown", handleKeydown);
  window.addEventListener("keyup", handleKeyup);
  window.addEventListener("wheel", handleWheel, { passive: false });
  window.addEventListener("blur", releaseSpacePTT);
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      releaseSpacePTT();
    }
  });
  window.addEventListener("beforeunload", shutdownAudio);
}

async function refreshState() {
  try {
    const response = await fetch("/api/state", { headers: { Accept: "application/json" } });
    const nextState = await response.json();
    applyState(nextState);
  } catch (error) {
    announce("Unable to refresh state.");
  }
}

function sendCommand(payload, announcementBuilder, focusRequest = null) {
  state.commandChain = state.commandChain
    .catch(() => undefined)
    .then(() => sendCommandNow(payload, announcementBuilder, focusRequest));

  return state.commandChain;
}

async function sendCommandNow(payload, announcementBuilder, focusRequest) {
  if (focusRequest) {
    state.pendingFocus = focusRequest;
  }

  try {
    logDebug("command", commandSummary(payload));
    const response = await fetch("/api/commands", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    const nextState = await response.json();
    applyState(nextState);
    logDebug("state", stateSummaryForDebug(nextState));

    if (announcementBuilder) {
      announce(announcementBuilder(nextState));
    }

    return nextState;
  } catch (error) {
    state.pendingFocus = null;
    logDebug("error", error && error.message ? error.message : "Command failed.");
    announce("Command failed.");
    throw error;
  }
}

function applyState(nextState) {
  state.current = nextState;
  state.shortcuts = nextState.shortcuts || [];
  state.diagnostics = nextState.diagnostics || null;

  renderApp(nextState.app, nextState.radio);
  renderDevices(nextState.devices, nextState.radio.device);
  renderRadio(nextState.radio, nextState.bands, nextState.modes, nextState.powerLevels);
  renderSettings(nextState.settings);
  renderShortcuts(nextState.shortcuts);
  renderDebug();
  syncAudioState(nextState.radio);
}

function renderApp(app, radioState) {
  elements.mode.textContent = app.activeMode;
  elements.remote.textContent = app.remoteEndpoint || "local only";
  elements.transport.textContent = app.proxyHealthy ? "reachable" : "degraded";
  elements.appDevice.textContent = radioState.device ? `${radioState.device.model} @ ${radioState.device.address}` : "No radio selected";
}

function renderDevices(devices, activeDevice) {
  elements.deviceList.innerHTML = "";

  if (!devices.length) {
    const empty = document.createElement("p");
    empty.className = "empty-state";
    empty.textContent = "No radios are listed. Run discovery to search the local network.";
    elements.deviceList.appendChild(empty);
    return;
  }

  devices.forEach((device) => {
    const wrapper = document.createElement("article");
    wrapper.className = "device-card";
    wrapper.setAttribute("role", "listitem");

    if (activeDevice && activeDevice.id === device.id) {
      wrapper.classList.add("is-connected");
    }

    const title = document.createElement("h3");
    title.textContent = `${device.model} at ${device.address}`;
    wrapper.appendChild(title);

    const detail = document.createElement("p");
    detail.textContent = `${device.protocol}, ${device.supportedReceivers} receiver paths, interface ${device.interfaceName}`;
    wrapper.appendChild(detail);

    const button = document.createElement("button");
    button.type = "button";
    button.textContent = activeDevice && activeDevice.id === device.id ? "Connected" : "Connect";
    button.disabled = Boolean(activeDevice && activeDevice.id === device.id);
    button.addEventListener("click", () => {
      sendCommand({ type: "connect", deviceId: device.id }, () => `${device.model} selected.`);
    });
    wrapper.appendChild(button);

    elements.deviceList.appendChild(wrapper);
  });
}

function renderRadio(radioState, bands, modes, powerLevels) {
  elements.connectionState.textContent = radioState.connected ? "Connected" : "Disconnected";
  elements.connectionState.classList.toggle("is-live", radioState.connected);
  elements.connectionState.classList.toggle("is-offline", !radioState.connected);

  elements.stationStatus.textContent = radioState.status;
  elements.hardwareSummary.textContent = radioState.capabilities.summary;
  elements.currentFrequency.textContent = radioState.frequencyMHz;
  elements.currentBand.textContent = radioState.bandLabel;
  elements.currentMode.textContent = radioState.modeLabel;
  elements.currentStep.textContent = formatStep(radioState.stepHz);
  elements.currentPower.textContent = radioState.powerLabel || `${radioState.powerPercent} percent drive`;
  elements.currentRadioState.textContent = radioStateSummary(radioState);

  syncSelectOptions(elements.powerSelect, powerLevels, (level) => ({
    value: String(level.percent),
    label: level.label,
  }));

  renderBandButtons(bands, radioState.bandId);
  renderModeButtons(modes, radioState.modeId);

  syncFrequencyInput(radioState);
  elements.stepSelect.value = String(radioState.stepHz);
  elements.powerSelect.value = String(radioState.powerPercent);

  setToggleState(elements.rxToggle, radioState.rxEnabled, "Receive");
  setToggleState(elements.txToggle, radioState.txEnabled, "Transmit armed");
  setToggleState(elements.pttToggle, radioState.ptt, "PTT");
  elements.pttToggle.disabled = !radioState.txEnabled;

  if (!radioState.ptt) {
    state.spacePTTActive = false;
  }
}

function renderBandButtons(bands, activeBandId) {
  renderChoiceButtons({
    container: elements.bandButtons,
    records: bands,
    activeId: activeBandId,
    groupName: "band",
    className: "band-button",
    getLabel: (band) => band.label,
    getDescription: (band) => `${band.label} band`,
    onActivate: (band, focusAfterRender) => {
      sendCommand(
        { type: "setBand", bandId: band.id },
        null,
        focusAfterRender ? { group: "band", value: band.id } : null
      );
    },
  });
}

function renderModeButtons(modes, activeModeId) {
  renderChoiceButtons({
    container: elements.modeButtons,
    records: modes,
    activeId: activeModeId,
    groupName: "mode",
    className: "mode-button",
    getLabel: (mode) => mode.label,
    getDescription: (mode) => mode.description,
    onActivate: (mode, focusAfterRender) => {
      sendCommand(
        { type: "setMode", modeId: mode.id },
        (nextState) => `${nextState.radio.modeLabel} mode.`,
        focusAfterRender ? { group: "mode", value: mode.id } : null
      );
    },
  });
}

function renderChoiceButtons({ container, records, activeId, groupName, className, getLabel, getDescription, onActivate }) {
  container.innerHTML = "";

  records.forEach((record, index) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = className;
    button.dataset.group = groupName;
    button.dataset.value = record.id;
    button.setAttribute("role", "radio");

    const active = record.id === activeId;
    button.textContent = getLabel(record);
    button.title = getDescription(record);
    button.setAttribute("aria-checked", active ? "true" : "false");
    button.tabIndex = active || (!activeId && index === 0) ? 0 : -1;

    if (active) {
      button.classList.add("is-active");
    }

    button.addEventListener("click", () => {
      onActivate(record, false);
    });

    button.addEventListener("keydown", (event) => {
      handleChoiceGroupKeydown(event, records, index, groupName, onActivate);
    });

    container.appendChild(button);

    if (state.pendingFocus && state.pendingFocus.group === groupName && state.pendingFocus.value === record.id) {
      window.requestAnimationFrame(() => button.focus());
      state.pendingFocus = null;
    }
  });
}

function handleChoiceGroupKeydown(event, records, index, groupName, onActivate) {
  let nextIndex = index;

  switch (event.key) {
    case "ArrowLeft":
    case "ArrowUp":
      nextIndex = (index - 1 + records.length) % records.length;
      break;
    case "ArrowRight":
    case "ArrowDown":
      nextIndex = (index + 1) % records.length;
      break;
    case "Home":
      nextIndex = 0;
      break;
    case "End":
      nextIndex = records.length - 1;
      break;
    default:
      return;
  }

  event.preventDefault();
  onActivate(records[nextIndex], true);
  state.pendingFocus = { group: groupName, value: records[nextIndex].id };
}

function renderSettings(settings) {
  syncInputValue(elements.settingsMode, settings.mode);
  syncInputValue(elements.settingsListen, settings.listenAddress);
  syncInputValue(elements.settingsRemote, settings.remoteBaseUrl);
  syncCheckboxValue(elements.settingsAccessibility, settings.accessibilityMode);
  elements.liveStatus.setAttribute("aria-live", settings.accessibilityMode ? "polite" : "off");
}

function renderShortcuts(shortcuts) {
  elements.shortcuts.innerHTML = "";
  shortcuts.forEach((shortcut) => {
    const item = document.createElement("li");
    item.className = "shortcut";

    const keys = document.createElement("strong");
    keys.textContent = shortcut.keys;
    item.appendChild(keys);

    const description = document.createElement("span");
    description.textContent = shortcut.description;
    item.appendChild(description);

    elements.shortcuts.appendChild(item);
  });
}

function syncSelectOptions(select, records, mapRecord) {
  if (select.dataset.loaded === "true") return;

  records.forEach((record) => {
    const option = document.createElement("option");
    const mapped = mapRecord(record);
    option.value = mapped.value;
    option.textContent = mapped.label;
    select.appendChild(option);
  });

  select.dataset.loaded = "true";
}

function syncInputValue(input, value) {
  if (document.activeElement === input) {
    return;
  }

  if (input.value !== value) {
    input.value = value;
  }
}

function syncCheckboxValue(input, checked) {
  if (document.activeElement === input) {
    return;
  }

  if (input.checked !== checked) {
    input.checked = checked;
  }
}

function setToggleState(button, enabled, label) {
  button.setAttribute("aria-pressed", enabled ? "true" : "false");
  button.classList.toggle("is-active", enabled);
  button.textContent = enabled ? `${label}: on` : `${label}: off`;
}

function syncFrequencyInput(radioState, force = false) {
  if (!force && document.activeElement === elements.frequencyInput) {
    return;
  }

  if (elements.frequencyInput.value !== radioState.frequencyMHz) {
    elements.frequencyInput.value = radioState.frequencyMHz;
  }
}

function handleKeydown(event) {
  if (!state.current) return;
  if (event.altKey || event.ctrlKey || event.metaKey) return;
  if (event.defaultPrevented) return;

  if (state.debugOpen) {
    handleDebugKeydown(event);
    return;
  }

  if (state.settingsOpen) {
    handleSettingsKeydown(event);
    return;
  }

  if (event.code === "Space") {
    handlePTTKeydown(event);
    return;
  }

  if (isEditableTarget(event.target)) return;

  switch (event.key.toLowerCase()) {
    case "p":
      event.preventDefault();
      sendCommand({ type: "cyclePower" }, (nextState) => `Power ${nextState.radio.powerLabel}.`);
      break;
    case "b":
      event.preventDefault();
      if (event.shiftKey) {
        announce(currentBandAnnouncement(state.current.radio));
        break;
      }
      sendCommand(
        { type: "cycleBand" },
        null,
        shouldPreserveGroupFocus(event.target, "band") ? { group: "band", value: nextBandRecord().id } : null
      );
      break;
    case "f":
      if (!event.shiftKey) {
        break;
      }
      event.preventDefault();
      announce(frequencyAnnouncement(state.current.radio));
      break;
    case "m":
      event.preventDefault();
      sendCommand(
        { type: "cycleMode" },
        (nextState) => `${nextState.radio.modeLabel} mode.`,
        shouldPreserveGroupFocus(event.target, "mode") ? { group: "mode", value: nextModeRecord().id } : null
      );
      break;
    case "s":
      event.preventDefault();
      openSettings();
      break;
    case "h":
      event.preventDefault();
      announce(shortcutsAnnouncement());
      break;
    case "d":
      event.preventDefault();
      openDebug();
      break;
    case "[":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? -10 : -1 },
        (nextState) => deferredFrequencyAnnouncement(nextState.radio)
      );
      break;
    case "]":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? 10 : 1 },
        (nextState) => deferredFrequencyAnnouncement(nextState.radio)
      );
      break;
    case "arrowup":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? 10 : 1 },
        (nextState) => deferredFrequencyAnnouncement(nextState.radio)
      );
      break;
    case "arrowdown":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? -10 : -1 },
        (nextState) => deferredFrequencyAnnouncement(nextState.radio)
      );
      break;
    case "r":
      event.preventDefault();
      sendCommand(
        { type: "setRX", enabled: !state.current.radio.rxEnabled },
        (nextState) => `Receive ${nextState.radio.rxEnabled ? "on" : "off"}.`
      );
      break;
    case "t":
      event.preventDefault();
      sendCommand(
        { type: "setTX", enabled: !state.current.radio.txEnabled },
        (nextState) => `Transmit ${nextState.radio.txEnabled ? "armed" : "disarmed"}.`
      );
      break;
    default:
      break;
  }
}

function handleKeyup(event) {
  if (event.code !== "Space") return;
  if (!state.spacePTTActive) return;
  event.preventDefault();
  releaseSpacePTT();
}

function handleSettingsKeydown(event) {
  if (event.key === "Escape") {
    event.preventDefault();
    closeSettings();
    return;
  }

  if (event.key === "Tab") {
    trapSettingsFocus(event);
    return;
  }

  if (isEditableTarget(event.target)) {
    return;
  }

  if (event.key.toLowerCase() === "h") {
    event.preventDefault();
    announce(shortcutsAnnouncement());
    return;
  }

  if (event.key.toLowerCase() === "b" && event.shiftKey) {
    event.preventDefault();
    announce(currentBandAnnouncement(state.current.radio));
    return;
  }

  if (event.key.toLowerCase() === "f" && event.shiftKey) {
    event.preventDefault();
    announce(frequencyAnnouncement(state.current.radio));
  }
}

function handleDebugKeydown(event) {
  if (event.key === "Escape") {
    event.preventDefault();
    closeDebug();
    return;
  }

  if (event.key === "Tab") {
    trapDialogFocus(event, elements.debugDialog);
  }
}

function openDebug() {
  if (state.debugOpen) return;

  state.debugOpen = true;
  state.lastDebugFocusedElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  elements.main.inert = true;
  elements.main.setAttribute("aria-hidden", "true");
  elements.debugModal.hidden = false;
  elements.openDebugButton.setAttribute("aria-expanded", "true");
  renderDebug();

  const focusable = getDialogFocusableElements(elements.debugDialog);
  if (focusable.length) {
    focusable[0].focus();
  } else {
    elements.debugDialog.focus();
  }
  announce("Debug console opened.");
}

function closeDebug() {
  if (!state.debugOpen) return;

  state.debugOpen = false;
  elements.debugModal.hidden = true;
  elements.main.inert = false;
  elements.main.removeAttribute("aria-hidden");
  elements.openDebugButton.setAttribute("aria-expanded", "false");

  const restoreTarget = state.lastDebugFocusedElement && state.lastDebugFocusedElement.isConnected
    ? state.lastDebugFocusedElement
    : elements.openDebugButton;
  state.lastDebugFocusedElement = null;
  restoreTarget.focus();
}

function openSettings() {
  if (state.settingsOpen) return;

  state.settingsOpen = true;
  state.lastFocusedElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  elements.main.inert = true;
  elements.main.setAttribute("aria-hidden", "true");
  elements.settingsModal.hidden = false;
  elements.openSettingsButton.setAttribute("aria-expanded", "true");
  const focusable = getSettingsFocusableElements();
  if (focusable.length) {
    focusable[0].focus();
  } else {
    elements.settingsDialog.focus();
  }
  announce("Settings opened.");
}

function closeSettings() {
  if (!state.settingsOpen) return;

  state.settingsOpen = false;
  elements.settingsModal.hidden = true;
  elements.main.inert = false;
  elements.main.removeAttribute("aria-hidden");
  elements.openSettingsButton.setAttribute("aria-expanded", "false");

  const restoreTarget = state.lastFocusedElement && state.lastFocusedElement.isConnected
    ? state.lastFocusedElement
    : elements.openSettingsButton;
  state.lastFocusedElement = null;
  restoreTarget.focus();
}

function trapSettingsFocus(event) {
  trapDialogFocus(event, elements.settingsDialog);
}

function trapDialogFocus(event, dialog) {
  const focusable = getDialogFocusableElements(dialog);
  if (!focusable.length) return;

  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const active = document.activeElement;

  if (event.shiftKey) {
    if (active === first || active === dialog) {
      event.preventDefault();
      last.focus();
    }
    return;
  }

  if (active === last) {
    event.preventDefault();
    first.focus();
  }
}

function getSettingsFocusableElements() {
  return getDialogFocusableElements(elements.settingsDialog);
}

function getDialogFocusableElements(dialog) {
  return Array.from(dialog.querySelectorAll(
    'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  ));
}

function handleWheel(event) {
  if (!state.current) return;
  if (state.settingsOpen) return;
  if (isEditableTarget(event.target)) return;
  if (event.ctrlKey || event.metaKey) return;

  event.preventDefault();

  state.wheelDeltaAccumulator += normalizeWheelDelta(event);
  scheduleWheelFlush();
}

function scheduleWheelFlush() {
  if (state.wheelFlushTimer !== null) return;

  state.wheelFlushTimer = window.setTimeout(() => {
    state.wheelFlushTimer = null;
    flushWheelTune();
  }, 35);
}

function flushWheelTune() {
  const notchSize = 100;
  const wholeNotches = truncateTowardZero(state.wheelDeltaAccumulator / notchSize);
  if (wholeNotches === 0) return;

  state.wheelDeltaAccumulator -= wholeNotches * notchSize;

  sendCommand(
    { type: "nudgeFrequency", steps: -wholeNotches },
    (nextState) => deferredFrequencyAnnouncement(nextState.radio)
  );
}

function normalizeWheelDelta(event) {
  let delta = event.deltaY;

  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
    delta *= 40;
  } else if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
    delta *= window.innerHeight;
  }

  return delta;
}

function truncateTowardZero(value) {
  if (value < 0) {
    return Math.ceil(value);
  }
  return Math.floor(value);
}

function handlePTTKeydown(event) {
  if (isInteractiveTarget(event.target)) return;

  event.preventDefault();
  if (event.repeat || state.spacePTTActive) return;

  if (!state.current.radio.txEnabled) {
    announce("Enable transmit before keying PTT.");
    return;
  }

  state.spacePTTActive = true;
  sendCommand({ type: "setPTT", enabled: true }, () => pttAnnouncement(true));
}

function releaseSpacePTT() {
  if (!state.current || !state.spacePTTActive) return;
  state.spacePTTActive = false;
  sendCommand({ type: "setPTT", enabled: false }, () => pttAnnouncement(false));
}

function shouldPreserveGroupFocus(target, groupName) {
  return Boolean(target && target.closest(`[data-group="${groupName}"]`));
}

function nextBandRecord() {
  const bands = state.current?.bands || [];
  const activeIndex = bands.findIndex((band) => band.id === state.current?.radio.bandId);
  return bands[(activeIndex + 1 + bands.length) % bands.length] || bands[0];
}

function nextModeRecord() {
  const modes = state.current?.modes || [];
  const activeIndex = modes.findIndex((mode) => mode.id === state.current?.radio.modeId);
  return modes[(activeIndex + 1 + modes.length) % modes.length] || modes[0];
}

function isEditableTarget(target) {
  if (!target) return false;
  const tagName = target.tagName ? target.tagName.toLowerCase() : "";
  return tagName === "input" || tagName === "select" || tagName === "textarea" || target.isContentEditable;
}

function isInteractiveTarget(target) {
  if (!target) return false;
  const tagName = target.tagName ? target.tagName.toLowerCase() : "";
  return tagName === "button" || tagName === "input" || tagName === "select" || tagName === "textarea" || target.isContentEditable;
}

function radioStateSummary(radioState) {
  const parts = [];
  parts.push(radioState.rxEnabled ? "RX on" : "RX off");
  parts.push(radioState.txEnabled ? "TX armed" : "TX safe");
  if (radioState.ptt) {
    parts.push("PTT live");
  }
  return parts.join(" / ");
}

function frequencyAnnouncement(radioState) {
  return `Frequency ${formatFrequencyForAnnouncement(radioState.frequencyMHz)} megahertz.`;
}

function currentBandAnnouncement(radioState) {
  return `Current band, ${spokenBandLabel(radioState.bandLabel)}.`;
}

function deferredFrequencyAnnouncement(radioState) {
  return {
    text: frequencyAnnouncement(radioState),
    speak: true,
    live: true,
    cue: null,
    delayMs: 750,
    key: "frequency",
  };
}

function shortcutsAnnouncement() {
  if (!state.shortcuts.length) {
    return "Shortcut list is not available yet.";
  }

  const parts = state.shortcuts.map((shortcut) => `${spokenShortcutKeys(shortcut.keys)}, ${shortcut.description}.`);
  return `Keyboard shortcuts. ${parts.join(" ")}`;
}

function spokenShortcutKeys(keys) {
  return keys
    .replace(/Shift \+ \[/g, "Shift plus left bracket")
    .replace(/Shift \+ \]/g, "Shift plus right bracket")
    .replace(/Shift \+ Arrow Up/g, "Shift plus arrow up")
    .replace(/Shift \+ Arrow Down/g, "Shift plus arrow down")
    .replace(/Shift \+ B/g, "Shift plus B")
    .replace(/Shift \+ F/g, "Shift plus F")
    .replace(/\[/g, "left bracket")
    .replace(/\]/g, "right bracket")
    .replace(/Arrow Up/g, "arrow up")
    .replace(/Arrow Down/g, "arrow down")
    .replace(/Hold Space/g, "hold space")
    .replace(/Wheel/g, "mouse wheel");
}

function spokenBandLabel(label) {
  return label.replace(/\bm$/i, "meters");
}

function formatFrequencyForAnnouncement(frequencyMHz) {
  const text = String(frequencyMHz ?? "").trim();
  const match = text.match(/^(\d+)(?:\.(\d+))?$/);
  if (!match) {
    return text;
  }

  const integerPart = match[1];
  const fractionalPart = match[2] || "";
  if (!fractionalPart) {
    return integerPart;
  }

  const minimumPrecision = Math.min(3, fractionalPart.length);
  const trimmedFractionLength = fractionalPart.replace(/0+$/, "").length;
  const precision = Math.max(minimumPrecision, trimmedFractionLength);
  return `${integerPart}.${fractionalPart.slice(0, precision)}`;
}

function formatStep(stepHz) {
  if (stepHz >= 1000) {
    return `${(stepHz / 1000).toFixed(stepHz % 1000 === 0 ? 0 : 1)} kHz`;
  }
  return `${stepHz} Hz`;
}

function startAudioFromUserGesture() {
  const audioContext = ensureAudioContext();
  if (!audioContext) {
    logAudioError("Web Audio is unavailable in this desktop webview.");
    announce("Audio engine is not available.");
    return;
  }

  resumeAudioContext();
  const radio = state.current?.radio;
  if (radio?.connected) {
    ensureRXAudio();
    if (radio.txEnabled) {
      ensureTXAudio();
    }
  }

  updateAudioDiagnostics();
  logDebug("audio", `manual audio start requested; context ${state.frontendDiagnostics.audioContextState}`);
  announce(radio?.connected ? "Audio start requested." : "Audio engine ready. Connect a radio to start receive audio.");
}

function syncAudioState(radioState) {
  const shouldReceive = Boolean(radioState.connected && radioState.rxEnabled && radioState.capabilities.rxAudioReady);
  if (shouldReceive) {
    ensureRXAudio();
  } else {
    stopRXAudio();
  }

  const shouldCapture = Boolean(radioState.connected && radioState.txEnabled && radioState.capabilities.txAudioReady);
  if (shouldCapture) {
    ensureTXAudio();
  } else {
    stopTXAudio();
  }
}

function ensureRXAudio() {
  if (state.rxSocket && (state.rxSocket.readyState === WebSocket.OPEN || state.rxSocket.readyState === WebSocket.CONNECTING)) {
    ensureRXPlayback();
    return;
  }

  const socket = new WebSocket(websocketURL("/api/audio/rx"));
  logDebug("audio", "opening RX audio websocket");
  socket.binaryType = "arraybuffer";
  socket.addEventListener("open", () => {
    logDebug("audio", "RX audio websocket open");
    ensureRXPlayback();
    resumeAudioContext();
    updateAudioDiagnostics();
  });
  socket.addEventListener("message", (event) => {
    if (!(event.data instanceof ArrayBuffer)) return;
    const frame = new Float32Array(event.data);
    state.rxBuffers.push(frame);
    state.frontendDiagnostics.rxFramesReceived++;
    state.frontendDiagnostics.rxSamplesReceived += frame.length;
    updateAudioDiagnostics();
  });
  socket.addEventListener("close", () => {
    if (state.rxSocket === socket) {
      logDebug("audio", "RX audio websocket closed");
      state.rxSocket = null;
      clearRXBuffers();
      updateAudioDiagnostics();
    }
  });
  socket.addEventListener("error", () => {
    logAudioError("RX audio websocket error");
    if (state.rxSocket === socket) {
      state.rxSocket = null;
    }
    updateAudioDiagnostics();
  });

  state.rxSocket = socket;
}

function ensureRXPlayback() {
  if (state.rxNode) return;

  const audioContext = ensureAudioContext();
  if (!audioContext) return;

  const processor = audioContext.createScriptProcessor(1024, 0, 2);
  processor.onaudioprocess = (event) => {
    state.frontendDiagnostics.rxPlaybackCallbacks++;
    const left = event.outputBuffer.getChannelData(0);
    const right = event.outputBuffer.getChannelData(1);
    fillOutputBuffer(left);
    right.set(left);
  };
  processor.connect(audioContext.destination);
  state.rxNode = processor;
  updateAudioDiagnostics();
  logDebug("audio", "RX playback processor connected");
}

function fillOutputBuffer(target) {
  let written = 0;

  while (written < target.length) {
    const current = state.rxBuffers[0];
    if (!current) {
      target.fill(0, written);
      state.frontendDiagnostics.rxUnderruns++;
      return;
    }

    const available = current.length - state.rxBufferOffset;
    const count = Math.min(available, target.length - written);
    target.set(current.subarray(state.rxBufferOffset, state.rxBufferOffset + count), written);

    written += count;
    state.rxBufferOffset += count;
    if (state.rxBufferOffset >= current.length) {
      state.rxBuffers.shift();
      state.rxBufferOffset = 0;
    }
  }
}

async function ensureTXAudio() {
  if (state.txSocket && (state.txSocket.readyState === WebSocket.OPEN || state.txSocket.readyState === WebSocket.CONNECTING) && state.txNode) {
    return;
  }

  const audioContext = ensureAudioContext();
  if (!audioContext || !navigator.mediaDevices || typeof navigator.mediaDevices.getUserMedia !== "function") {
    logAudioError("Microphone capture is unavailable in this desktop webview.");
    return;
  }

  if (!state.txSocket || state.txSocket.readyState > WebSocket.OPEN) {
    const socket = new WebSocket(websocketURL("/api/audio/tx"));
    logDebug("audio", "opening TX audio websocket");
    socket.binaryType = "arraybuffer";
    socket.addEventListener("open", () => {
      logDebug("audio", "TX audio websocket open");
      updateAudioDiagnostics();
    });
    socket.addEventListener("close", () => {
      if (state.txSocket === socket) {
        logDebug("audio", "TX audio websocket closed");
        state.txSocket = null;
        updateAudioDiagnostics();
      }
    });
    socket.addEventListener("error", () => {
      logAudioError("TX audio websocket error");
      if (state.txSocket === socket) {
        state.txSocket = null;
      }
      updateAudioDiagnostics();
    });
    state.txSocket = socket;
  }

  if (state.micStream && state.txNode) {
    resumeAudioContext();
    return;
  }

  try {
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
      },
    });

    const source = audioContext.createMediaStreamSource(stream);
    const processor = audioContext.createScriptProcessor(1024, 1, 1);
    const silentMonitor = audioContext.createGain();
    silentMonitor.gain.value = 0;

    processor.onaudioprocess = (event) => {
      const radioState = state.current?.radio;
      if (!radioState || !radioState.ptt) return;
      if (!state.txSocket || state.txSocket.readyState !== WebSocket.OPEN) return;

      const input = event.inputBuffer.getChannelData(0);
      state.txSocket.send(floatsToArrayBuffer(input));
      state.frontendDiagnostics.txFramesSent++;
      state.frontendDiagnostics.txSamplesSent += input.length;
      updateAudioDiagnostics();
    };

    source.connect(processor);
    processor.connect(silentMonitor);
    silentMonitor.connect(audioContext.destination);

    state.micStream = stream;
    state.micSource = source;
    state.txNode = processor;
    state.txMonitor = silentMonitor;
    state.lastMicError = "";
    state.frontendDiagnostics.micState = "capturing";
    resumeAudioContext();
    updateAudioDiagnostics();
    logDebug("audio", "microphone capture connected");
  } catch (error) {
    const message = error && error.message ? error.message : "Microphone access failed.";
    state.frontendDiagnostics.micState = "failed";
    logAudioError(message);
    if (state.lastMicError !== message) {
      state.lastMicError = message;
      announce(`Microphone unavailable. ${message}`);
    }
  }
}

function stopRXAudio() {
  if (state.rxSocket) {
    state.rxSocket.close();
    state.rxSocket = null;
  }
  if (state.rxNode) {
    state.rxNode.disconnect();
    state.rxNode = null;
  }
  clearRXBuffers();
  updateAudioDiagnostics();
}

function stopTXAudio() {
  if (state.txSocket) {
    state.txSocket.close();
    state.txSocket = null;
  }
  if (state.txNode) {
    state.txNode.disconnect();
    state.txNode = null;
  }
  if (state.txMonitor) {
    state.txMonitor.disconnect();
    state.txMonitor = null;
  }
  if (state.micSource) {
    state.micSource.disconnect();
    state.micSource = null;
  }
  if (state.micStream) {
    state.micStream.getTracks().forEach((track) => track.stop());
    state.micStream = null;
  }
  state.frontendDiagnostics.micState = "stopped";
  updateAudioDiagnostics();
}

function shutdownAudio() {
  stopRXAudio();
  stopTXAudio();
}

function clearRXBuffers() {
  state.rxBuffers = [];
  state.rxBufferOffset = 0;
}

function websocketURL(path) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}${path}`;
}

function floatsToArrayBuffer(input) {
  const buffer = new ArrayBuffer(input.length * 4);
  const view = new Float32Array(buffer);
  view.set(input);
  return buffer;
}

function updateAudioDiagnostics() {
  state.frontendDiagnostics.audioContextState = state.audioContext ? state.audioContext.state : "not-created";
  state.frontendDiagnostics.rxSocketState = websocketState(state.rxSocket);
  state.frontendDiagnostics.txSocketState = websocketState(state.txSocket);
  if (state.micStream && state.micStream.active) {
    state.frontendDiagnostics.micState = "capturing";
  } else if (!state.micStream && state.frontendDiagnostics.micState === "capturing") {
    state.frontendDiagnostics.micState = "stopped";
  }
  renderDebug();
}

function websocketState(socket) {
  if (!socket) return "closed";
  switch (socket.readyState) {
    case WebSocket.CONNECTING:
      return "connecting";
    case WebSocket.OPEN:
      return "open";
    case WebSocket.CLOSING:
      return "closing";
    case WebSocket.CLOSED:
      return "closed";
    default:
      return "unknown";
  }
}

function logAudioError(message) {
  state.frontendDiagnostics.lastAudioError = message;
  logDebug("audio-error", message);
  updateAudioDiagnostics();
}

function logDebug(kind, text) {
  const entry = {
    time: new Date().toLocaleTimeString(),
    kind,
    text,
  };
  state.debugEvents.unshift(entry);
  if (state.debugEvents.length > 80) {
    state.debugEvents.length = 80;
  }
  renderDebug();
}

function renderDebug() {
  if (!elements.debugRadio || !elements.debugAudio || !elements.debugLog) return;
  if (!state.debugOpen && elements.debugModal?.hidden) return;

  const diagnostics = state.diagnostics || {};
  renderMetrics(elements.debugRadio, [
    ["Connected", yesNo(diagnostics.connected)],
    ["Transport", diagnostics.transport || "none"],
    ["Local socket", diagnostics.localAddress || "none"],
    ["Radio socket", diagnostics.remoteAddress || "none"],
    ["Started", diagnostics.startedAt || "not started"],
    ["Last control", diagnostics.lastControl || "none"],
    ["Last error", diagnostics.lastError || "none"],
    ["Send packets", diagnostics.sendPackets || 0],
    ["Start / stop", `${diagnostics.startCommands || 0} / ${diagnostics.stopCommands || 0}`],
    ["Control frames", diagnostics.controlFrames || 0],
    ["Frequency frames", diagnostics.frequencyFrames || 0],
    ["Last TX freq", formatHz(diagnostics.lastTxFrequencyHz)],
    ["Last RX freq", formatHz(diagnostics.lastRxFrequencyHz)],
    ["RX packets", diagnostics.rxPackets || 0],
    ["RX frames", diagnostics.rxFrames || 0],
    ["RX audio frames", diagnostics.rxAudioFrames || 0],
    ["RX audio samples", diagnostics.rxAudioSamples || 0],
    ["RX drops", diagnostics.rxAudioDrops || 0],
    ["RX subscribers", diagnostics.rxSubscribers || 0],
    ["TX audio frames", diagnostics.txAudioFrames || 0],
    ["TX audio samples", diagnostics.txAudioSamples || 0],
    ["TX IQ samples", diagnostics.txIqSamples || 0],
  ]);

  const audio = state.frontendDiagnostics;
  renderMetrics(elements.debugAudio, [
    ["Audio context", audio.audioContextState],
    ["RX socket", audio.rxSocketState],
    ["TX socket", audio.txSocketState],
    ["RX frames received", audio.rxFramesReceived],
    ["RX samples received", audio.rxSamplesReceived],
    ["Playback callbacks", audio.rxPlaybackCallbacks],
    ["Playback underruns", audio.rxUnderruns],
    ["Buffered RX chunks", state.rxBuffers.length],
    ["TX frames sent", audio.txFramesSent],
    ["TX samples sent", audio.txSamplesSent],
    ["Microphone", audio.micState],
    ["Last audio error", audio.lastAudioError || "none"],
  ]);

  elements.debugLog.innerHTML = "";
  state.debugEvents.forEach((entry) => {
    const item = document.createElement("li");
    item.className = "debug-log-entry";
    item.textContent = `${entry.time} [${entry.kind}] ${entry.text}`;
    elements.debugLog.appendChild(item);
  });
}

function renderMetrics(container, metrics) {
  container.innerHTML = "";
  metrics.forEach(([label, value]) => {
    const row = document.createElement("div");
    const term = document.createElement("dt");
    const detail = document.createElement("dd");
    term.textContent = label;
    detail.textContent = String(value);
    row.appendChild(term);
    row.appendChild(detail);
    container.appendChild(row);
  });
}

function commandSummary(payload) {
  const parts = [payload.type];
  if (payload.bandId) parts.push(`band=${payload.bandId}`);
  if (payload.modeId) parts.push(`mode=${payload.modeId}`);
  if (payload.frequencyMHz) parts.push(`freq=${payload.frequencyMHz} MHz`);
  if (payload.stepHz) parts.push(`step=${payload.stepHz}`);
  if (payload.steps) parts.push(`steps=${payload.steps}`);
  if (payload.powerPercent) parts.push(`power=${payload.powerPercent}%`);
  if (typeof payload.enabled === "boolean") parts.push(`enabled=${payload.enabled}`);
  return parts.join(" ");
}

function stateSummaryForDebug(nextState) {
  const radio = nextState.radio;
  return `${radio.bandLabel} ${radio.frequencyMHz} MHz ${radio.modeLabel}; hardware=${yesNo(radio.capabilities.hardwareReady)} rxAudio=${yesNo(radio.capabilities.rxAudioReady)}`;
}

async function copyDebugReport() {
  const report = JSON.stringify({
    radio: state.current?.radio || null,
    app: state.current?.app || null,
    diagnostics: state.diagnostics || null,
    frontendAudio: state.frontendDiagnostics,
    recentEvents: state.debugEvents.slice(0, 30),
  }, null, 2);

  try {
    await navigator.clipboard.writeText(report);
    announce("Debug report copied.");
    logDebug("debug", "debug report copied to clipboard");
  } catch (error) {
    announce("Unable to copy debug report.");
    logDebug("error", error && error.message ? error.message : "clipboard copy failed");
  }
}

function yesNo(value) {
  return value ? "yes" : "no";
}

function formatHz(value) {
  if (!value) return "none";
  return `${value} Hz`;
}

function announce(text) {
  const announcement = normalizeAnnouncement(text);
  if (!announcement) return;

  if (announcement.delayMs > 0) {
    queueDelayedAnnouncement(announcement);
    return;
  }

  clearDelayedAnnouncement();
  deliverAnnouncement(announcement);
}

function deliverAnnouncement(announcement) {
  if (announcement.cue) {
    playCue(announcement.cue);
  }

  if (announcement.text) {
    updateLiveStatus(announcement.text, { announceToAT: announcement.live !== false });
  }

  if (!announcement.text || announcement.speak === false || !elements.settingsAccessibility.checked) return;

  const now = Date.now();
  if (state.lastAnnouncementText === announcement.text && now - state.lastAnnouncementAt < 800) {
    return;
  }
  state.lastAnnouncementText = announcement.text;
  state.lastAnnouncementAt = now;

  if (window.go && window.go.desktop && window.go.desktop.AccessibilityBridge && typeof window.go.desktop.AccessibilityBridge.Announce === "function") {
    window.go.desktop.AccessibilityBridge.Announce(announcement.text).catch((error) => {
      console.error("native announce failed", error);
    });
  }
}

function queueDelayedAnnouncement(announcement) {
  clearDelayedAnnouncement();
  state.delayedAnnouncement = announcement;
  state.delayedAnnouncementTimer = window.setTimeout(() => {
    const pending = state.delayedAnnouncement;
    clearDelayedAnnouncement();
    if (pending) {
      deliverAnnouncement(pending);
    }
  }, announcement.delayMs);
}

function clearDelayedAnnouncement() {
  if (state.delayedAnnouncementTimer !== null) {
    window.clearTimeout(state.delayedAnnouncementTimer);
    state.delayedAnnouncementTimer = null;
  }
  state.delayedAnnouncement = null;
}

function normalizeAnnouncement(value) {
  if (!value) return null;
  if (typeof value === "string") {
    return { text: value, speak: true, live: true, cue: null, delayMs: 0, key: "" };
  }
  return {
    text: value.text || "",
    speak: value.speak !== false,
    live: value.live !== false,
    cue: value.cue || null,
    delayMs: Number(value.delayMs) > 0 ? Number(value.delayMs) : 0,
    key: value.key || "",
  };
}

function pttAnnouncement(enabled) {
  return {
    text: enabled ? "PTT on." : "PTT off.",
    speak: false,
    live: false,
    cue: enabled ? "ptt-on" : "ptt-off",
  };
}

function updateLiveStatus(text, options = {}) {
  const announceToAT = options.announceToAT !== false;
  const restoreLiveMode = elements.settingsAccessibility.checked ? "polite" : "off";

  if (!announceToAT) {
    elements.liveStatus.setAttribute("aria-live", "off");
  }

  elements.liveStatus.textContent = "";
  window.requestAnimationFrame(() => {
    elements.liveStatus.textContent = text;

    if (!announceToAT) {
      window.setTimeout(() => {
        elements.liveStatus.setAttribute("aria-live", restoreLiveMode);
      }, 0);
    }
  });
}

function playCue(cue) {
  let audioContext;
  try {
    audioContext = ensureAudioContext();
  } catch (error) {
    console.error("audio context unavailable", error);
    return;
  }

  if (!audioContext) return;

  const pattern = cue === "ptt-on"
    ? [
        { frequency: 880, duration: 0.05, gain: 0.05 },
        { frequency: 1320, duration: 0.08, gain: 0.05 },
      ]
    : [
        { frequency: 720, duration: 0.05, gain: 0.05 },
        { frequency: 480, duration: 0.08, gain: 0.05 },
      ];

  const startPlayback = () => {
    let when = audioContext.currentTime + 0.01;
    pattern.forEach((tone) => {
      when = scheduleTone(audioContext, tone, when);
    });
  };

  if (audioContext.state === "suspended") {
    audioContext.resume().then(startPlayback).catch((error) => {
      console.error("audio resume failed", error);
    });
    return;
  }

  startPlayback();
}

function ensureAudioContext() {
  if (state.audioContext) {
    updateAudioDiagnostics();
    return state.audioContext;
  }

  const AudioContextCtor = window.AudioContext || window.webkitAudioContext;
  if (!AudioContextCtor) {
    return null;
  }

  try {
    state.audioContext = new AudioContextCtor({ sampleRate: 48000 });
  } catch (error) {
    state.audioContext = new AudioContextCtor();
  }
  updateAudioDiagnostics();
  return state.audioContext;
}

function resumeAudioContext() {
  const audioContext = ensureAudioContext();
  if (!audioContext || audioContext.state !== "suspended") return;

  audioContext.resume().catch((error) => {
    logAudioError(error && error.message ? error.message : "audio resume failed");
    console.error("audio resume failed", error);
  }).finally(() => {
    updateAudioDiagnostics();
  });
}

function scheduleTone(audioContext, tone, startTime) {
  const oscillator = audioContext.createOscillator();
  const gainNode = audioContext.createGain();
  const endTime = startTime + tone.duration;
  const fadeIn = Math.min(0.012, tone.duration / 3);
  const fadeOutStart = Math.max(startTime + fadeIn, endTime - 0.018);

  oscillator.type = "sine";
  oscillator.frequency.setValueAtTime(tone.frequency, startTime);

  gainNode.gain.setValueAtTime(0.0001, startTime);
  gainNode.gain.linearRampToValueAtTime(tone.gain, startTime + fadeIn);
  gainNode.gain.setValueAtTime(tone.gain, fadeOutStart);
  gainNode.gain.exponentialRampToValueAtTime(0.0001, endTime);

  oscillator.connect(gainNode);
  gainNode.connect(audioContext.destination);

  oscillator.start(startTime);
  oscillator.stop(endTime + 0.01);

  return endTime + 0.03;
}
