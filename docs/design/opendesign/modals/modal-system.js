/* AGH modal design-system behavior: dialog lifecycle, progressive disclosure,
   canonical selector mocks, catalog states, secrets, validation, and saving. */
(() => {
  const root = document;
  const body = document.body;
  const toast = root.querySelector('[data-toast]');
  const query = new URLSearchParams(window.location.search);
  const slug = (window.location.pathname.split('/').pop() || 'entity-editor').replace(/\.html$/, '');
  const usedInspectIds = new Set();
  if (toast) {
    toast.setAttribute('role', 'status');
    toast.setAttribute('aria-live', 'polite');
  }
  const inspectId = (base) => {
    const normalized = String(base || 'control').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'control';
    let candidate = `${slug}-${normalized}`;
    let index = 2;
    while (usedInspectIds.has(candidate)) candidate = `${slug}-${normalized}-${index++}`;
    usedInspectIds.add(candidate);
    return candidate;
  };

  root.querySelectorAll('[data-od-id]').forEach((element) => usedInspectIds.add(element.dataset.odId));
  root.querySelectorAll('.scrim .dialog').forEach((dialog) => {
    const title = dialog.querySelector('h1');
    const description = dialog.querySelector('.head-copy p');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    if (title) {
      if (!title.id) title.id = `${slug}-title`;
      dialog.setAttribute('aria-labelledby', title.id);
    }
    if (description) {
      if (!description.id) description.id = `${slug}-description`;
      dialog.setAttribute('aria-describedby', description.id);
    }
  });
  root.querySelectorAll('.product').forEach((surface) => {
    surface.setAttribute('aria-hidden', 'true');
    surface.inert = true;
  });
  root.querySelectorAll('header, section, footer, h1, h2, input, textarea, select, button, details, .field, .chip, .secret, .notice, .note, .switch-row, .modebar, .identity-row').forEach((element) => {
    if (element.dataset.odId) return;
    const label = element.id || element.getAttribute('aria-label') || element.textContent?.trim().slice(0, 48) || element.tagName;
    element.dataset.odId = inspectId(label);
  });
  root.querySelectorAll('.field').forEach((field, index) => {
    const error = field.querySelector('.error');
    const control = field.querySelector('input, textarea, select');
    const label = field.querySelector('label, legend, .label');
    if (control && label && !control.getAttribute('aria-label')) {
      const hasAssociation = control.id && label.getAttribute('for') === control.id;
      if (!hasAssociation && !label.contains(control)) control.setAttribute('aria-label', label.textContent.trim());
    }
    if (!error || !control) return;
    if (!error.id) error.id = `${slug}-field-error-${index + 1}`;
    error.setAttribute('role', 'alert');
    control.setAttribute('aria-describedby', [control.getAttribute('aria-describedby'), error.id].filter(Boolean).join(' '));
  });
  const notify = (message) => {
    if (!toast) return;
    toast.textContent = message;
    toast.dataset.open = 'true';
    toast.classList.add('show');
    window.clearTimeout(window.__aghToastTimer);
    window.__aghToastTimer = window.setTimeout(() => {
      toast.dataset.open = 'false';
      toast.classList.remove('show');
    }, 3200);
  };
  window.aghNotify = notify;

  const activeDialog = root.querySelector('.scrim .dialog');
  const closedState = root.querySelector('.closed-state');
  const closeDialog = () => {
    body.dataset.closed = 'true';
    activeDialog?.setAttribute('aria-hidden', 'true');
    activeDialog?.closest('.scrim')?.setAttribute('hidden', '');
    if (closedState) {
      closedState.classList.add('is-visible');
      closedState.tabIndex = -1;
      closedState.focus();
    }
  };
  root.querySelectorAll('[data-close]').forEach((button) => button.addEventListener('click', closeDialog));
  root.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && body.dataset.closed !== 'true' && !root.querySelector('.component-popover[data-open="true"]')) closeDialog();
    if (event.key !== 'Tab' || !activeDialog || body.dataset.closed === 'true') return;
    const focusable = [...activeDialog.querySelectorAll('button:not(:disabled), input:not(:disabled), textarea:not(:disabled), select:not(:disabled), summary, [tabindex]:not([tabindex="-1"])')].filter((element) => !element.hidden && element.offsetParent !== null);
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && root.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && root.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });
  window.requestAnimationFrame(() => {
    const initial = activeDialog?.querySelector('[autofocus]') || activeDialog?.querySelector('input:not(:disabled), textarea:not(:disabled), select:not(:disabled), button:not([data-close]):not(:disabled), [data-close]');
    initial?.focus();
  });

  /* Popover/listbox mocks mirror CommandSelect and RuntimeSelector focus ownership. */
  const openPopovers = new Set();
  let syncMemberCoordinator = () => {};
  const createChip = (list, value, form) => {
    if (!list || !value || [...list.querySelectorAll('.chip')].some((chip) => chip.dataset.value === value)) return;
    const chip = root.createElement('span');
    chip.className = 'chip';
    chip.dataset.value = value;
    chip.dataset.odId = inspectId(`member-${value}`);
    chip.append(root.createTextNode(value));
    const remove = root.createElement('button');
    remove.type = 'button';
    remove.setAttribute('aria-label', `Remove ${value}`);
    remove.dataset.odId = inspectId(`remove-${value}`);
    remove.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';
    remove.addEventListener('click', () => {
      chip.remove();
      const trigger = form?.querySelector(`[data-chip-target="${list.id}"]`);
      const popover = trigger && root.getElementById(trigger.getAttribute('aria-controls') || '');
      popover?.querySelectorAll('[role="option"]').forEach((option) => {
        if ((option.dataset.value || option.textContent.trim()) === value) option.setAttribute('aria-selected', 'false');
      });
      if (trigger && popover) syncMultiTrigger(trigger, popover, false);
      syncMemberCoordinator(form);
    });
    chip.append(remove);
    list.append(chip);
  };
  const syncMultiTrigger = (trigger, popover, renderChips = true) => {
    const selected = [...popover.querySelectorAll('[role="option"][aria-selected="true"]')];
    const values = selected.map((option) => option.dataset.value || option.textContent.trim());
    const count = values.length;
    const pluralNoun = trigger.dataset.selectionNoun || 'items';
    const noun = count === 1 ? (trigger.dataset.selectionNounSingular || pluralNoun.replace(/s$/, '')) : pluralNoun;
    const avatar = trigger.querySelector('.command-select__avatar');
    const copy = trigger.querySelector('[data-component-value]');
    const summary = trigger.querySelector('[data-component-summary]');
    if (avatar) avatar.textContent = String(count);
    if (copy) copy.textContent = `${count} ${noun} selected`;
    if (summary) summary.textContent = values.length ? values.join(', ') : (trigger.dataset.emptyCopy || 'Nothing selected');
    trigger.setAttribute('aria-label', `${count} ${noun} selected`);
    const list = trigger.dataset.chipTarget && root.getElementById(trigger.dataset.chipTarget);
    if (renderChips && list) {
      list.replaceChildren();
      values.forEach((value) => createChip(list, value, trigger.closest('form')));
      syncMemberCoordinator(trigger.closest('form'));
    }
  };
  const closePopover = (popover, trigger, restoreFocus = true) => {
    popover.dataset.open = 'false';
    popover.setAttribute('aria-hidden', 'true');
    root.querySelectorAll(`[aria-controls="${popover.id}"]`).forEach((owner) => owner.setAttribute('aria-expanded', 'false'));
    openPopovers.delete(popover);
    if (restoreFocus) trigger.focus();
  };
  root.querySelectorAll('[data-component-trigger]').forEach((trigger) => {
    const popover = root.getElementById(trigger.getAttribute('aria-controls') || '');
    if (!popover) return;
    trigger.setAttribute('aria-haspopup', trigger.dataset.componentTrigger === 'runtime-selector' ? 'dialog' : 'listbox');
    trigger.setAttribute('aria-expanded', 'false');
    popover.setAttribute('aria-hidden', 'true');
    trigger.addEventListener('click', () => {
      const opening = popover.dataset.open !== 'true';
      [...openPopovers].forEach((open) => {
        const owner = open.__aghOpeningTrigger || root.querySelector(`[aria-controls="${open.id}"]`);
        if (owner) closePopover(open, owner, false);
      });
      if (!opening) return closePopover(popover, trigger);
      popover.__aghOpeningTrigger = trigger;
      const rect = trigger.getBoundingClientRect();
      popover.dataset.open = 'true';
      popover.setAttribute('aria-hidden', 'false');
      popover.style.left = `${Math.max(12, Math.min(rect.left, window.innerWidth - Math.min(528, window.innerWidth - 24) - 12))}px`;
      popover.style.top = `${Math.min(rect.bottom + 6, window.innerHeight - Math.min(popover.scrollHeight || 320, 520) - 12)}px`;
      trigger.setAttribute('aria-expanded', 'true');
      openPopovers.add(popover);
      window.requestAnimationFrame(() => popover.querySelector('input, [role="option"], button')?.focus());
    });
    if (trigger.dataset.componentTrigger === 'agent-multi-select' || trigger.hasAttribute('data-multi-select')) syncMultiTrigger(trigger, popover, false);
    if (popover.dataset.behaviorBound === 'true') return;
    popover.dataset.behaviorBound = 'true';
    popover.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        closePopover(popover, popover.__aghOpeningTrigger || trigger);
        return;
      }
      const options = [...popover.querySelectorAll('[role="option"]')].filter((option) => !option.disabled && !option.hidden);
      const current = options.indexOf(root.activeElement);
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault();
        if (!options.length) return;
        if (current === -1) {
          options[event.key === 'ArrowDown' ? 0 : options.length - 1].focus();
          return;
        }
        const offset = event.key === 'ArrowDown' ? 1 : -1;
        options[(current + offset + options.length) % options.length]?.focus();
      }
    });
    popover.querySelector('input')?.addEventListener('input', (event) => {
      const queryText = event.target.value.trim().toLowerCase();
      popover.querySelectorAll('[role="option"]').forEach((option) => {
        option.hidden = queryText !== '' && !option.textContent.toLowerCase().includes(queryText);
      });
    });
    popover.addEventListener('click', (event) => {
      const option = event.target.closest('[role="option"]');
      if (!option || !popover.contains(option)) return;
      const owner = popover.__aghOpeningTrigger || trigger;
      const multi = owner.dataset.componentTrigger === 'agent-multi-select' || owner.hasAttribute('data-multi-select');
      popover.querySelectorAll('[role="option"]').forEach((item) => {
        if (!multi) item.setAttribute('aria-selected', String(item === option));
      });
      if (multi) option.setAttribute('aria-selected', String(option.getAttribute('aria-selected') !== 'true'));
      const valueTrigger = owner.dataset.componentTrigger === 'runtime-selector'
        ? root.querySelector(`[aria-controls="${popover.id}"][data-runtime-segment="model"]`) || owner
        : owner;
      const copy = valueTrigger.querySelector('[data-component-value]');
      if (multi) syncMultiTrigger(owner, popover);
      else if (copy) copy.textContent = option.dataset.value || option.textContent.trim();
      if (!multi && owner.hasAttribute('data-member-coordinator')) owner.dataset.value = option.dataset.value || option.textContent.trim();
      if (!multi) closePopover(popover, owner);
      owner.dispatchEvent(new Event('change', { bubbles: true }));
    });
  });
  root.querySelectorAll('.component-popover [role="radiogroup"]').forEach((group) => {
    const radios = [...group.querySelectorAll('[role="radio"]')];
    const activate = (radio) => {
      radios.forEach((item) => {
        item.setAttribute('aria-checked', String(item === radio));
        item.tabIndex = item === radio ? 0 : -1;
      });
    };
    radios.forEach((radio, index) => {
      radio.addEventListener('click', () => activate(radio));
      radio.addEventListener('keydown', (event) => {
        if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return;
        event.preventDefault();
        const offset = ['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1;
        const next = event.key === 'Home' ? radios[0] : event.key === 'End' ? radios.at(-1) : radios[(index + offset + radios.length) % radios.length];
        next.focus();
        activate(next);
      });
    });
    const selected = radios.find((radio) => radio.getAttribute('aria-checked') === 'true') || radios[0];
    if (selected) activate(selected);
  });
  root.querySelectorAll('[data-pressed-group]').forEach((group) => {
    const buttons = [...group.querySelectorAll('button[aria-pressed]')];
    const activate = (button) => {
      buttons.forEach((item) => item.setAttribute('aria-pressed', String(item === button)));
      const reasoningTrigger = root.querySelector('[data-runtime-segment="reasoning"] strong');
      if (reasoningTrigger) reasoningTrigger.textContent = button.textContent.trim();
    };
    buttons.forEach((button) => button.addEventListener('click', () => activate(button)));
  });
  root.querySelectorAll('[data-favorite-key]').forEach((button) => button.addEventListener('click', () => {
    const pressed = button.getAttribute('aria-pressed') !== 'true';
    button.setAttribute('aria-pressed', String(pressed));
    button.querySelector('[data-favorite-copy]')?.replaceChildren(root.createTextNode(pressed ? 'Favorited' : 'Add to favorites'));
    notify(`${button.dataset.favoriteKey} ${pressed ? 'added to' : 'removed from'} favorites.`);
  }));

  /* Simple / Advanced uses the production PillGroup button-group contract. */
  const modeButtons = [...root.querySelectorAll('[data-mode]')];
  const activateMode = (button) => {
    const mode = button.dataset.mode;
    modeButtons.forEach((item) => item.setAttribute('aria-pressed', String(item === button)));
    root.querySelectorAll('.advanced').forEach((item) => { item.hidden = mode !== 'advanced'; });
    root.querySelectorAll('[data-adv]').forEach((item) => item.classList.toggle('hide', mode !== 'advanced'));
    root.dispatchEvent(new CustomEvent('agh:mode', { detail: { mode } }));
  };
  modeButtons.forEach((button) => {
    button.addEventListener('click', () => activateMode(button));
  });
  if (modeButtons.length) activateMode(modeButtons.find((b) => b.getAttribute('aria-pressed') === 'true') || modeButtons[0]);

  const setConditionalState = (element, visible) => {
    element.hidden = !visible;
    element.querySelectorAll('input, textarea, select, button').forEach((control) => {
      if (control.matches('[data-required-when-visible]')) control.required = visible;
      control.disabled = !visible;
    });
  };

  /* PillGroup: role=group + pressed buttons, matching the production primitive. */
  root.querySelectorAll('[data-pillgroup]').forEach((group) => {
    group.setAttribute('role', 'group');
    if (!group.getAttribute('aria-label')) {
      const label = group.closest('.field')?.querySelector('.label, label, legend');
      if (label) group.setAttribute('aria-label', label.textContent.trim());
    }
    const pills = [...group.querySelectorAll('.pill')];
    const activate = (pill) => {
      pills.forEach((item) => {
        item.setAttribute('aria-pressed', String(item === pill));
      });
      const name = group.dataset.pillgroup;
      root.querySelectorAll(`[data-conditional="${name}"]`).forEach((item) => {
        setConditionalState(item, item.dataset.match === pill.dataset.value);
      });
      root.dispatchEvent(new CustomEvent('agh:pill', { detail: { group: name, value: pill.dataset.value } }));
    };
    pills.forEach((pill) => {
      pill.addEventListener('click', () => activate(pill));
    });
    const selected = pills.find((p) => p.getAttribute('aria-pressed') === 'true') || pills[0];
    if (selected) activate(selected);
  });

  /* choice-card groups (template cards) — radio semantics + reveal */
  root.querySelectorAll('[data-choice-group]').forEach((group) => {
    group.setAttribute('role', 'radiogroup');
    if (!group.getAttribute('aria-label') && !group.getAttribute('aria-labelledby')) {
      const label = group.closest('.field')?.querySelector('.label, label, legend') || group.closest('.sec')?.querySelector('.sec__title');
      if (label) group.setAttribute('aria-label', label.textContent.trim());
    }
    const choices = [...group.querySelectorAll('[data-choice]')];
    const activateChoice = (button) => {
      choices.forEach((item) => {
        item.setAttribute('role', 'radio');
        item.setAttribute('aria-checked', String(item === button));
        item.setAttribute('aria-pressed', String(item === button));
        item.tabIndex = item === button ? 0 : -1;
      });
      const target = button.dataset.reveal;
      root.querySelectorAll(`[data-conditional="${group.dataset.choiceGroup}"]`).forEach((item) => {
        setConditionalState(item, item.id === target || item.dataset.match === target);
      });
      root.dispatchEvent(new CustomEvent('agh:choice', { detail: { group: group.dataset.choiceGroup, value: target || button.dataset.value || '' , button } }));
    };
    choices.forEach((button, index) => {
      button.addEventListener('click', () => activateChoice(button));
      button.addEventListener('keydown', (event) => {
        if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
        event.preventDefault();
        const offset = ['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1;
        const next = choices[(index + offset + choices.length) % choices.length];
        next.focus();
        activateChoice(next);
      });
    });
    const selected = group.querySelector('[data-choice][aria-checked="true"], [data-choice][aria-pressed="true"]') || choices[0];
    if (selected) activateChoice(selected);
  });

  const permissionNotes = {
    'perm-deny': 'Maps to deny all. Every tool call pauses for approval before it runs.',
    'perm-reads': 'Maps to approve reads. Sessions read the workspace freely and pause on writes, commands, and network calls.',
    'perm-all': 'Maps to approve all. Sessions act without approval gates; pair this policy with a sandbox profile.'
  };
  const bridgeCredentialCopy = {
    'slack-secrets': 'Slack declares two write-only slots.',
    'discord-secrets': 'Discord declares two write-only slots.'
  };
  root.addEventListener('agh:choice', (event) => {
    if (event.detail.group === 'permission') {
      const note = root.getElementById('perm-note-txt');
      if (note && permissionNotes[event.detail.value]) note.textContent = permissionNotes[event.detail.value];
    }
    if (event.detail.group === 'provider') {
      const sub = root.getElementById('cred-sub');
      if (sub && bridgeCredentialCopy[event.detail.value]) sub.textContent = bridgeCredentialCopy[event.detail.value];
    }
  });

  root.querySelectorAll('[data-agent-catalog]').forEach((control) => control.addEventListener('change', () => {
    const avatar = root.getElementById('agent-av');
    if (avatar) avatar.textContent = (control.value || control.textContent || '?').trim().charAt(0).toLowerCase();
  }));

  root.querySelectorAll('[data-reveal-select]').forEach((select) => {
    const sync = () => {
      root.querySelectorAll(`[data-conditional="${select.dataset.revealSelect}"]`).forEach((item) => {
        setConditionalState(item, item.dataset.match === select.value);
      });
      root.dispatchEvent(new CustomEvent('agh:select', { detail: { name: select.dataset.revealSelect, value: select.value } }));
    };
    select.addEventListener('change', sync);
    sync();
  });

  /* provider → model catalog wiring (status banners removed from modal chrome) */
  root.querySelectorAll('[data-provider-catalog]').forEach((provider) => provider.addEventListener('change', () => {
    const model = root.querySelector('[data-model-catalog]');
    if (!model) return;
    model.disabled = true;
    window.setTimeout(() => {
      model.disabled = false;
      model.innerHTML = provider.value === 'openai'
        ? '<option value="">Select model</option><option>gpt-5</option><option>gpt-5-mini</option>'
        : provider.value === 'anthropic'
          ? '<option value="">Select model</option><option>claude-sonnet-4</option><option>claude-opus-4</option>'
          : '<option value="">Agent default</option>';
    }, 420);
  }));

  /* switches */
  root.querySelectorAll('.switch').forEach((button) => button.addEventListener('click', () => {
    const checked = button.getAttribute('aria-checked') !== 'true';
    button.setAttribute('aria-checked', String(checked));
    const target = button.dataset.secretRequiredToggle && root.querySelector(button.dataset.secretRequiredToggle);
    if (target) {
      if (checked) target.dataset.secretRequired = '';
      else delete target.dataset.secretRequired;
    }
    root.dispatchEvent(new CustomEvent('agh:switch', { detail: { button, checked } }));
  }));

  /* vault reference collision */
  root.querySelectorAll('[data-ref-check]').forEach((input) => {
    const collision = root.querySelector('[data-ref-collision]');
    const sync = () => {
      if (!collision) return;
      collision.hidden = input.value.trim() !== input.dataset.refCheck;
      if (collision.hidden) collision.querySelector('.switch')?.setAttribute('aria-checked', 'false');
    };
    input.addEventListener('input', sync);
    sync();
  });

  /* write-only secret slots — Set value / Rotate / Cancel */
  root.querySelectorAll('[data-secret-action]').forEach((button) => button.addEventListener('click', () => {
    const secret = button.closest('.field, .settings-row')?.querySelector('.secret');
    if (!secret) return;
    const editing = secret.dataset.editing !== 'true';
    secret.dataset.editing = String(editing);
    if (!editing) secret.parentElement?.querySelectorAll('.secret__editor input').forEach((input) => { input.value = ''; });
    button.textContent = editing ? 'Cancel' : (button.dataset.secretAction || 'Set value');
    if (editing) secret.parentElement?.querySelector('.secret__editor input')?.focus();
  }));

  /* chips (members, directories) */
  root.querySelectorAll('[data-add-chip]').forEach((button) => button.addEventListener('click', () => {
    const field = button.closest('.field');
    const input = field?.querySelector('input, select');
    const list = field?.querySelector('.chip-list');
    const value = input?.value.trim();
    if (!value || !list) return;
    if ([...list.querySelectorAll('.chip')].some((chip) => chip.dataset.value === value)) {
      notify('Already in the list.');
      return;
    }
    createChip(list, value, button.closest('form'));
    if (input.tagName === 'SELECT') input.selectedIndex = 0; else input.value = '';
    syncMemberCoordinator(button.closest('form'));
  }));

  syncMemberCoordinator = (form) => {
    if (!form) return;
    const list = form.querySelector('[data-members-list]');
    const coordinator = form.querySelector('[data-member-coordinator]');
    if (!list || !coordinator) return;
    const current = coordinator.dataset.value || '';
    const members = [...list.querySelectorAll('.chip')].map((chip) => chip.dataset.value || chip.firstChild?.textContent?.trim()).filter(Boolean);
    const popover = root.getElementById(coordinator.getAttribute('aria-controls') || '');
    const optionList = popover?.querySelector('.component-popover__list');
    if (!optionList) return;
    optionList.replaceChildren();
    members.forEach((member) => {
      const option = root.createElement('button');
      option.className = 'component-popover__option';
      option.type = 'button';
      option.setAttribute('role', 'option');
      option.setAttribute('aria-selected', String(member === current));
      option.dataset.value = member;
      option.textContent = member;
      optionList.append(option);
    });
    if (!members.includes(current)) {
      coordinator.dataset.value = '';
      const copy = coordinator.querySelector('[data-component-value]');
      if (copy) copy.textContent = 'Select coordinator';
    }
  };
  root.querySelectorAll('[data-members-list] .chip').forEach((chip) => {
    chip.dataset.value ||= chip.firstChild?.textContent?.trim() || '';
  });
  root.querySelectorAll('.chip button').forEach((button) => button.addEventListener('click', () => {
    const form = button.closest('form');
    button.closest('.chip')?.remove();
    syncMemberCoordinator(form);
  }));
  root.querySelectorAll('form').forEach(syncMemberCoordinator);

  /* native CLI auth diagnostics */
  root.querySelectorAll('[data-auth-refresh]').forEach((button) => button.addEventListener('click', () => {
    const status = button.closest('[data-auth-status]');
    const label = status?.querySelector('[data-auth-status-copy]');
    if (!status || !label) return;
    button.disabled = true;
    label.textContent = 'Checking native CLI login…';
    window.setTimeout(() => {
      const failed = query.get('auth') === 'error';
      button.disabled = false;
      label.textContent = failed ? 'Native CLI status unavailable. Retry without losing changes.' : 'Native CLI authenticated and command available.';
      status.classList.toggle('notice--danger', failed);
      status.classList.toggle('notice--success', !failed);
    }, 620);
  }));

  /* bridge delivery test */
  root.querySelectorAll('[data-delivery-test]').forEach((button) => button.addEventListener('click', () => {
    const status = root.querySelector('[data-delivery-status]');
    const original = button.textContent;
    button.disabled = true;
    button.textContent = 'Testing…';
    if (status) status.textContent = 'Sending through the bridge test endpoint…';
    window.setTimeout(() => {
      const failed = query.get('delivery') === 'error';
      button.disabled = false;
      button.textContent = failed ? 'Retry test' : original;
      if (status) status.textContent = failed ? 'Delivery failed. Review the endpoint response and retry.' : 'Test delivery accepted by the bridge endpoint.';
      notify(failed ? 'Delivery test failed.' : 'Delivery test completed.');
    }, 720);
  }));

  /* edit surfaces: dirty tracking gates the primary action */
  root.querySelectorAll('form').forEach((form) => {
    if (!slug.includes('-edit')) return;
    const submit = form.querySelector('[type="submit"]');
    const note = form.querySelector('.foot-note span[data-note], .foot-note');
    const markDirty = () => {
      form.dataset.dirty = 'true';
      if (submit) submit.disabled = false;
      if (note) note.textContent = 'Unsaved changes are kept until the daemon accepts them.';
    };
    submit && (submit.disabled = true);
    form.addEventListener('input', markDirty);
    form.addEventListener('change', markDirty);
    form.addEventListener('click', (event) => {
      if (event.target.closest('.switch, [data-choice], .pill, [data-secret-action], [data-add-chip], .chip button')) markDirty();
    });
  });

  /* submit — contract-shaped validation, saving state, draft-preserving errors */
  root.querySelectorAll('form').forEach((form) => form.addEventListener('submit', (event) => {
    event.preventDefault();
    let invalid = false;
    form.querySelectorAll('[required]:not(:disabled)').forEach((input) => {
      const field = input.closest('.field');
      const missing = !String(input.value).trim();
      if (field) field.dataset.invalid = String(missing);
      invalid ||= missing;
    });
    form.querySelectorAll('[data-secret-required]:not(:disabled)').forEach((input) => {
      const field = input.closest('.field, .settings-row');
      const missing = !String(input.value).trim();
      if (field) field.dataset.invalid = String(missing);
      if (missing) field?.querySelector('.secret')?.setAttribute('data-editing', 'true');
      invalid ||= missing;
    });
    form.querySelectorAll('[data-safe-json]:not(:disabled)').forEach((input) => {
      const field = input.closest('.field');
      let unsafe = false;
      if (input.value.trim()) {
        try {
          const payload = JSON.parse(input.value);
          const visit = (value) => {
            if (!value || typeof value !== 'object') return;
            for (const [key, nested] of Object.entries(value)) {
              if (/(secret|token|password|credential|api[_-]?key)/i.test(key)) unsafe = true;
              visit(nested);
            }
          };
          visit(payload);
        } catch {
          unsafe = true;
        }
      }
      if (field) field.dataset.invalid = String(unsafe);
      invalid ||= unsafe;
    });
    form.querySelectorAll('[data-absolute-path]:not(:disabled)').forEach((input) => {
      const field = input.closest('.field');
      const invalidPath = Boolean(input.value.trim()) && !/^(\/|[A-Za-z]:[\\/])/.test(input.value.trim());
      if (field) field.dataset.invalid = String(invalidPath);
      invalid ||= invalidPath;
    });
    form.querySelectorAll('[data-env-name]:not(:disabled)').forEach((input) => {
      const field = input.closest('.field, .settings-row');
      const invalidName = Boolean(input.value.trim()) && !/^[A-Z_][A-Z0-9_]*$/.test(input.value.trim());
      if (field) field.dataset.invalid = String(invalidName);
      invalid ||= invalidName;
    });
    form.querySelectorAll('[data-secret-ref]:not(:disabled)').forEach((input) => {
      const field = input.closest('.field, .settings-row');
      const invalidRef = Boolean(input.value.trim()) && !/^(?:[a-zA-Z][a-zA-Z0-9+.-]*:)?[a-zA-Z0-9][a-zA-Z0-9._/-]*$/.test(input.value.trim());
      if (field) field.dataset.invalid = String(invalidRef);
      invalid ||= invalidRef;
    });
    form.querySelectorAll('[data-members-list][data-required-members]').forEach((list) => {
      const empty = !list.querySelector('.chip');
      const field = list.closest('.field');
      if (field) field.dataset.invalid = String(empty);
      invalid ||= empty;
    });
    form.querySelectorAll('[data-required-selection]:not(:disabled)').forEach((control) => {
      const missing = !control.dataset.value;
      const field = control.closest('.field');
      if (field) field.dataset.invalid = String(missing);
      invalid ||= missing;
    });
    const replacement = form.querySelector('[data-ref-collision]:not([hidden]) .switch');
    if (replacement && replacement.getAttribute('aria-checked') !== 'true') {
      invalid = true;
      notify('Confirm replacement for the existing secret reference.');
    }
    if (invalid) {
      form.querySelector('[data-invalid="true"] input, [data-invalid="true"] textarea, [data-invalid="true"] select')?.focus();
      notify('Complete the required fields.');
      return;
    }
    const submit = form.querySelector('[type="submit"]');
    if (submit) {
      submit.disabled = true;
      submit.classList.add('is-loading');
      submit.dataset.label = submit.textContent;
      submit.textContent = 'Saving…';
    }
    window.setTimeout(() => {
      const failed = query.get('save') === 'error';
      if (submit) {
        submit.disabled = false;
        submit.classList.remove('is-loading');
        submit.textContent = failed ? 'Retry save' : (submit.dataset.label || 'Save');
      }
      if (failed) {
        notify('Save failed. Your draft is still here.');
        return;
      }
      form.dataset.dirty = 'false';
      if (slug.includes('-edit') && submit) submit.disabled = true;
      const note = form.querySelector('.foot-note');
      if (note && slug.includes('-edit')) note.textContent = 'Saved. No pending changes.';
      notify(form.dataset.success || 'Ready to save.');
    }, 520);
  }));

  root.addEventListener('input', (event) => {
    const field = event.target.closest?.('.field, .settings-row');
    if (field && String(event.target.value).trim()) field.dataset.invalid = 'false';
  });

  root.querySelectorAll('[data-notify]').forEach((button) => button.addEventListener('click', () => notify(button.dataset.notify)));

  /* Optional Visual Contract isolator: ?preview=<name>&state=<state> when a host
     marks regions with data-preview (no current surface ships those markers;
     prefer STATE-MATRIX.md review flags + living popovers). */
  const previewName = query.get('preview');
  if (previewName) {
    const preview = root.querySelector(`[data-preview="${CSS.escape(previewName)}"]`);
    if (preview) {
      body.classList.add('preview-only');
      root.querySelectorAll('[data-preview]').forEach((item) => { item.hidden = item !== preview; });
      preview.hidden = false;
      preview.dataset.previewActive = 'true';
      const state = query.get('state') || 'default';
      preview.dataset.previewState = state;
      preview.querySelectorAll('[data-state-kind]').forEach((item) => { item.hidden = item.dataset.stateKind !== state; });
      if (state !== 'default') {
        window.requestAnimationFrame(() => {
          const trigger = preview.querySelector('[data-component-trigger]');
          const modelSegment = preview.querySelector('[data-runtime-segment="model"]');
          const stateLabel = preview.querySelector(`[data-state-kind="${CSS.escape(state)}"]`);
          if (state === 'no-model' && modelSegment) {
            modelSegment.querySelector('strong')?.replaceChildren(root.createTextNode('Select model'));
            modelSegment.setAttribute('aria-label', 'Model: Select model');
          }
          if (state === 'disabled') {
            preview.querySelectorAll('.runtime-trigger button').forEach((button) => { button.disabled = true; });
            preview.querySelector('.runtime-trigger')?.setAttribute('aria-disabled', 'true');
            if (stateLabel) stateLabel.hidden = false;
            return;
          }
          trigger?.click();
          const popover = root.getElementById(trigger?.getAttribute('aria-controls') || '');
          const list = popover?.querySelector('[role="listbox"]');
          const stateCopy = {
            loading: 'Loading models…',
            refreshing: 'Refreshing catalog…',
            empty: 'No runtime choices found. Configure a provider, then refresh.',
            'no-match': 'No models match this search.',
            error: 'Catalog unavailable. Retry without losing the draft.',
            'no-model': 'No model selected. Choose a model to complete the runtime.',
          };
          if (state === 'stale' && popover) {
            const message = root.createElement('div');
            message.className = 'component-popover__status';
            message.setAttribute('role', 'status');
            message.textContent = 'Catalog may be stale. Refresh before choosing an unavailable model.';
            popover.append(message);
          }
          if (list && stateCopy[state]) {
            list.setAttribute('aria-busy', String(['loading', 'refreshing'].includes(state)));
            const message = root.createElement('div');
            message.className = 'component-popover__option';
            message.setAttribute('role', 'status');
            message.textContent = stateCopy[state];
            list.replaceChildren(message);
          }
        });
      }
    }
  }

  /* DirectoryBrowser — transcribes web/src/systems/onboarding directory-browser.tsx:
     home/up toolbar with mono path, "Use this folder", hover-reveal row pick, and
     reading/empty states. A canned tree stands in for the daemon browse endpoint. */
  const directoryTree = {
    '/': ['Applications', 'Library', 'System', 'Users'],
    '/Users': ['you'],
    '/Users/you': ['Desktop', 'Dev', 'Documents', 'Downloads', 'Movies', 'Music', 'Pictures'],
    '/Users/you/Dev': ['agh-extensions', 'billing', 'checkout-platform', 'design-tokens', 'growth-experiments', 'shared-libs'],
    '/Users/you/Dev/agh-extensions': ['bridge-sdk', 'hooks', 'marketplace'],
    '/Users/you/Dev/billing': ['api', 'migrations', 'worker'],
    '/Users/you/Dev/checkout-platform': ['apps', 'docs', 'infra', 'packages', 'scripts'],
    '/Users/you/Dev/shared-libs': ['auth-kit', 'http-clients', 'telemetry'],
  };
  const directoryIcons = {
    folder: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>',
    folderPlus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/><path d="M12 10v6"/><path d="M9 13h6"/></svg>',
    close: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',
  };
  root.querySelectorAll('[data-od-component="directory-browser"]').forEach((browser) => {
    const pathLabel = browser.querySelector('[data-dir-path]');
    const list = browser.querySelector('[data-dir-list]');
    const homeButton = browser.querySelector('[data-dir-action="home"]');
    const upButton = browser.querySelector('[data-dir-action="up"]');
    const useButton = browser.querySelector('[data-dir-action="use"]');
    const input = root.getElementById(browser.dataset.dirInput || '');
    const cardSlot = root.getElementById(browser.dataset.dirCard || '');
    const nameInput = root.getElementById(browser.dataset.dirName || '');
    if (!pathLabel || !list || !useButton || !input) return;
    const home = browser.dataset.dirHome || '/';
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    let current = browser.dataset.dirCurrent || home;
    let selected = input.value.trim();
    let firstRender = true;
    const baseName = (path) => (path === '/' ? '/' : path.slice(path.lastIndexOf('/') + 1));
    const parentOf = (path) => {
      if (path === '/') return null;
      const trimmed = path.slice(0, path.lastIndexOf('/'));
      return trimmed === '' ? '/' : trimmed;
    };
    const joinPath = (dir, name) => (dir === '/' ? `/${name}` : `${dir}/${name}`);
    const titleFromPath = (path) => {
      const words = baseName(path).replace(/[-_]+/g, ' ').trim();
      return words ? words.charAt(0).toUpperCase() + words.slice(1) : '';
    };
    let nameTouched = Boolean(nameInput && nameInput.value.trim()) && nameInput.value !== titleFromPath(selected);
    const renderToolbar = () => {
      pathLabel.textContent = current;
      pathLabel.title = current;
      if (upButton) upButton.disabled = parentOf(current) === null;
      useButton.disabled = Boolean(selected) && selected === current;
    };
    const renderState = (label, busy) => {
      const state = root.createElement('div');
      state.className = 'dir-browser__state';
      if (busy) {
        const spin = root.createElement('i');
        spin.className = 'spinner';
        state.append(spin);
      }
      state.append(root.createTextNode(label));
      list.setAttribute('aria-busy', String(Boolean(busy)));
      list.replaceChildren(state);
    };
    const renderList = () => {
      list.setAttribute('aria-busy', 'false');
      list.replaceChildren();
      const entries = directoryTree[current] || [];
      if (!entries.length) {
        renderState('No sub-folders here.', false);
        return;
      }
      entries.forEach((name, index) => {
        const entryPath = joinPath(current, name);
        const row = root.createElement('div');
        row.className = 'dir-row';
        row.dataset.odId = `${slug}-dir-entry-${name}`;
        const nav = root.createElement('button');
        nav.type = 'button';
        nav.className = 'dir-row__nav';
        nav.innerHTML = `${directoryIcons.folder}<span></span>`;
        nav.querySelector('span').textContent = name;
        nav.setAttribute('aria-label', `Open ${name}`);
        if (firstRender && index === 0) nav.setAttribute('autofocus', '');
        nav.addEventListener('click', () => navigateTo(entryPath));
        const pick = root.createElement('button');
        pick.type = 'button';
        pick.className = 'dir-row__pick';
        pick.innerHTML = directoryIcons.folderPlus;
        pick.setAttribute('aria-label', `Use ${name} as the workspace root`);
        pick.disabled = selected === entryPath;
        pick.addEventListener('click', () => selectRoot(entryPath));
        row.append(nav, pick);
        list.append(row);
      });
      firstRender = false;
    };
    const renderCard = () => {
      if (!cardSlot) return;
      cardSlot.replaceChildren();
      const card = root.createElement('div');
      if (!selected) {
        card.className = 'root-card root-card--empty';
        card.textContent = 'No folder selected yet — browse above and pick one.';
        cardSlot.append(card);
        return;
      }
      card.className = 'root-card';
      const well = root.createElement('span');
      well.className = 'root-card__well';
      well.setAttribute('aria-hidden', 'true');
      well.innerHTML = directoryIcons.folder;
      const copy = root.createElement('span');
      copy.className = 'root-card__copy';
      const title = root.createElement('strong');
      title.textContent = baseName(selected);
      const path = root.createElement('span');
      path.textContent = selected;
      copy.append(title, path);
      const clear = root.createElement('button');
      clear.type = 'button';
      clear.className = 'icon-button';
      clear.innerHTML = directoryIcons.close;
      clear.setAttribute('aria-label', 'Clear selected root');
      clear.addEventListener('click', () => {
        selected = '';
        input.value = '';
        input.dispatchEvent(new Event('input', { bubbles: true }));
        renderToolbar();
        renderList();
        renderCard();
      });
      card.append(well, copy, clear);
      cardSlot.append(card);
    };
    const navigateTo = (path) => {
      if (path === current) return;
      const go = () => {
        current = path;
        renderToolbar();
        renderList();
      };
      if (reduceMotion) {
        go();
        return;
      }
      renderState('Reading directory…', true);
      window.setTimeout(go, 180);
    };
    const selectRoot = (path) => {
      selected = path;
      input.value = path;
      const field = input.closest('.field');
      if (field) field.dataset.invalid = 'false';
      if (nameInput && !nameTouched) nameInput.value = titleFromPath(path);
      input.dispatchEvent(new Event('input', { bubbles: true }));
      renderToolbar();
      renderList();
      renderCard();
    };
    homeButton?.addEventListener('click', () => navigateTo(home));
    upButton?.addEventListener('click', () => {
      const parent = parentOf(current);
      if (parent) navigateTo(parent);
    });
    useButton.addEventListener('click', () => selectRoot(current));
    nameInput?.addEventListener('input', () => {
      nameTouched = nameInput.value.trim() !== '';
    });
    renderToolbar();
    renderList();
    renderCard();
  });
})();
