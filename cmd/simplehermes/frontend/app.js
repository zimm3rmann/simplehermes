const state = {
  current: null,
  pollTimer: null,
  commandChain: Promise.resolve(),
  pendingFocus: null,
  spacePTTActive: false,
  lastAnnouncementText: "",
  lastAnnouncementAt: 0,
  wheelDeltaAccumulator: 0,
  wheelFlushTimer: null,
  audioContext: null,
  settingsOpen: false,
  lastFocusedElement: null,
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

  elements.frequencyInput = document.getElementById("frequency-input");
  elements.stepSelect = document.getElementById("step-select");
  elements.powerSelect = document.getElementById("power-select");

  elements.rxToggle = document.getElementById("rx-toggle");
  elements.txToggle = document.getElementById("tx-toggle");
  elements.pttToggle = document.getElementById("ptt-toggle");

  elements.radioForm = document.getElementById("radio-form");
  elements.settingsForm = document.getElementById("settings-form");

  elements.settingsMode = document.getElementById("settings-mode");
  elements.settingsListen = document.getElementById("settings-listen");
  elements.settingsRemote = document.getElementById("settings-remote");
  elements.settingsAccessibility = document.getElementById("settings-accessibility");
}

function bindActions() {
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
        (nextState) => frequencyAnnouncement(nextState.radio)
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
    const response = await fetch("/api/commands", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    const nextState = await response.json();
    applyState(nextState);

    if (announcementBuilder) {
      announce(announcementBuilder(nextState));
    }

    return nextState;
  } catch (error) {
    state.pendingFocus = null;
    announce("Command failed.");
    throw error;
  }
}

function applyState(nextState) {
  state.current = nextState;

  renderApp(nextState.app, nextState.radio);
  renderDevices(nextState.devices, nextState.radio.device);
  renderRadio(nextState.radio, nextState.bands, nextState.modes, nextState.powerLevels);
  renderSettings(nextState.settings);
  renderShortcuts(nextState.shortcuts);
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
        (nextState) => bandAnnouncement(nextState.radio),
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
      sendCommand(
        { type: "cycleBand" },
        (nextState) => bandAnnouncement(nextState.radio),
        shouldPreserveGroupFocus(event.target, "band") ? { group: "band", value: nextBandRecord().id } : null
      );
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
    case "[":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? -10 : -1 },
        (nextState) => frequencyAnnouncement(nextState.radio)
      );
      break;
    case "]":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? 10 : 1 },
        (nextState) => frequencyAnnouncement(nextState.radio)
      );
      break;
    case "arrowup":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? 10 : 1 },
        (nextState) => frequencyAnnouncement(nextState.radio)
      );
      break;
    case "arrowdown":
      event.preventDefault();
      sendCommand(
        { type: "nudgeFrequency", steps: event.shiftKey ? -10 : -1 },
        (nextState) => frequencyAnnouncement(nextState.radio)
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

  if (event.key !== "Tab") {
    return;
  }

  trapSettingsFocus(event);
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
  const focusable = getSettingsFocusableElements();
  if (!focusable.length) return;

  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const active = document.activeElement;

  if (event.shiftKey) {
    if (active === first || active === elements.settingsDialog) {
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
  return Array.from(elements.settingsDialog.querySelectorAll(
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
    (nextState) => frequencyAnnouncement(nextState.radio)
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

function bandAnnouncement(radioState) {
  return `${spokenBandLabel(radioState.bandLabel)}, ${formatFrequencyForAnnouncement(radioState.frequencyMHz)} megahertz.`;
}

function frequencyAnnouncement(radioState) {
  return `Frequency ${formatFrequencyForAnnouncement(radioState.frequencyMHz)} megahertz.`;
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

function spokenBandLabel(label) {
  return label.replace(/\bm$/i, "meters");
}

function formatStep(stepHz) {
  if (stepHz >= 1000) {
    return `${(stepHz / 1000).toFixed(stepHz % 1000 === 0 ? 0 : 1)} kHz`;
  }
  return `${stepHz} Hz`;
}

function announce(text) {
  const announcement = normalizeAnnouncement(text);
  if (!announcement) return;

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

function normalizeAnnouncement(value) {
  if (!value) return null;
  if (typeof value === "string") {
    return { text: value, speak: true, live: true, cue: null };
  }
  return {
    text: value.text || "",
    speak: value.speak !== false,
    live: value.live !== false,
    cue: value.cue || null,
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
  if (state.audioContext) return state.audioContext;

  const AudioContextCtor = window.AudioContext || window.webkitAudioContext;
  if (!AudioContextCtor) {
    return null;
  }

  state.audioContext = new AudioContextCtor();
  return state.audioContext;
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
