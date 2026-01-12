((window) => {
	window.Fusionaly = window.Fusionaly || {};

	window.Fusionaly.config = window.Fusionaly.config || {
		host: "{{.BaseURL}}",
		eventTypes: {
			pageView: 1,
			customEvent: 2,
		},
		sendInterval: 200,
		maxRetries: 3,
		maxBatchSize: 10,
		userId: null,
		respectDoNotTrack: true,
		debug: false,
		autoInstrumentButtons: true,
		autoSendPageViews: true,
		scrollDepthThresholds: [25, 50, 75, 100],
		scrollDepthEventKey: "scroll:depth",
		scrollSectionEventKey: "scroll:section",
		scrollSectionThreshold: 0.5,
	};

	window.Fusionaly.config.scrollDepthThresholds =
		window.Fusionaly.config.scrollDepthThresholds || [25, 50, 75, 100];
	window.Fusionaly.config.scrollDepthEventKey =
		window.Fusionaly.config.scrollDepthEventKey || "scroll:depth";
	window.Fusionaly.config.scrollSectionEventKey =
		window.Fusionaly.config.scrollSectionEventKey || "scroll:section";
	window.Fusionaly.config.scrollSectionThreshold =
		typeof window.Fusionaly.config.scrollSectionThreshold === "number"
			? window.Fusionaly.config.scrollSectionThreshold
			: 0.5;

	const baseUrl = window.Fusionaly.config.host;
	let eventBuffer = [];
	let isOnline = navigator.onLine;
	const STORAGE_KEY = "fusionaly_pendingEvents";

	// Helper function for conditional logging
	const log = (message, level = "info") => {
		if (window.Fusionaly.config.debug) {
			if (level === "error") {
				console.error(`Fusionaly: ${message}`);
			} else {
				console.log(`Fusionaly: ${message}`);
			}
		} else if (level === "error") {
			console.error(`Fusionaly: ${message}`);
		}
	};

	// Helper function to check if tracking is allowed
	const shouldTrack = () => {
		return !(
			navigator.doNotTrack === "1" &&
			window.Fusionaly.config.respectDoNotTrack
		);
	};

	// Listen for online/offline events
	window.addEventListener("online", () => {
		isOnline = true;
		sendStoredEvents();
		log("Back online, sending stored events");
	});

	window.addEventListener("offline", () => {
		isOnline = false;
		log("Offline, events will be stored locally");
	});

	// Add SPA support by monitoring navigation
	const setupSPATracking = () => {
		const originalPushState = history.pushState;
		history.pushState = (...args) => {
			originalPushState.apply(this, args);
			if (window.Fusionaly.config.autoSendPageViews) {
				setTimeout(sendPageView, 50);
			}
		};

		window.addEventListener("popstate", () => {
			if (window.Fusionaly.config.autoSendPageViews) {
				setTimeout(sendPageView, 50);
			}
		});
	};

	const sendBufferedEvents = () => {
		if (eventBuffer.length > 0) {
			const eventsToSend = eventBuffer;
			eventBuffer = [];

			for (const eventData of eventsToSend) {
				sendEventWithRetry(eventData, 0);
			}
		}

		setTimeout(sendBufferedEvents, window.Fusionaly.config.sendInterval);
	};

	const sendEventWithRetry = (eventData, retryCount) => {
		if (!shouldTrack()) {
			return;
		}

		if (!isOnline) {
			storeEventLocally(eventData);
			return;
		}
		eventData.userAgent = navigator.userAgent;

		fetch(`${baseUrl}/x/api/v1/events`, {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
				Referer: document.referrer,
			},
			body: JSON.stringify(eventData),
		})
			.then((response) => {
				if (!response.ok) {
					log(
						`Error sending event: ${response.status} ${response.statusText}`,
						"error",
					);
					retry(eventData, retryCount);
				} else {
					response
						.json()
						.then((data) => log("Event sent successfully"))
						.catch((error) =>
							log(`Error parsing JSON response: ${error}`, "error"),
						);
				}
			})
			.catch((error) => {
				log(`Error sending event: ${error}`, "error");
				retry(eventData, retryCount);
			});
	};

	const retry = (eventData, retryCount) => {
		if (retryCount < window.Fusionaly.config.maxRetries) {
			const delay = 2 ** retryCount * 1000 * (0.8 + Math.random() * 0.4);
			log(`Retrying in ${Math.round(delay / 1000)} seconds...`);
			setTimeout(() => sendEventWithRetry(eventData, retryCount + 1), delay);
		} else {
			log("Max retries reached. Storing event locally", "error");
			storeEventLocally(eventData);
		}
	};

	const storeEventLocally = (eventData) => {
		try {
			const storedEvents = getStoredEvents();
			storedEvents.push(eventData);

			const maxStoredEvents = 100;
			if (storedEvents.length > maxStoredEvents) {
				storedEvents.splice(0, storedEvents.length - maxStoredEvents);
			}

			localStorage.setItem(STORAGE_KEY, JSON.stringify(storedEvents));
		} catch (e) {
			log(`Failed to store event locally: ${e}`, "error");
		}
	};

	const getStoredEvents = () => {
		try {
			const storedEvents = localStorage.getItem(STORAGE_KEY);
			return storedEvents ? JSON.parse(storedEvents) : [];
		} catch (e) {
			log(`Failed to retrieve stored events: ${e}`, "error");
			return [];
		}
	};

	const sendStoredEvents = () => {
		try {
			const storedEvents = getStoredEvents();
			if (storedEvents.length > 0) {
				log(`Attempting to send ${storedEvents.length} stored events`);

				localStorage.removeItem(STORAGE_KEY);
				for (const eventData of storedEvents) {
					bufferEvent(eventData);
				}
			}
		} catch (e) {
			log(`Failed to process stored events: ${e}`, "error");
		}
	};

	const bufferEvent = (eventData) => {
		eventBuffer.push(eventData);
	};

	const sendPageView = () => {
		if (!shouldTrack()) {
			return;
		}

		bufferEvent({
			timestamp: new Date().toISOString(),
			referrer: document.referrer,
			url: window.location.href,
			userId: window.Fusionaly.userId,
			eventType: window.Fusionaly.config.eventTypes.pageView,
		});
	};

	const registerPurchase = (priceInCents, currency = 'USD', metadata = {}) => {
		if (!shouldTrack()) {
			return;
		}

		// Validate price
		if (typeof priceInCents !== 'number' || priceInCents <= 0) {
			log('registerPurchase: price must be a positive number in cents', 'error');
			return;
		}

		// Prepare metadata with price in cents
		const purchaseMetadata = {
			price: priceInCents,
			currency: currency,
			quantity: metadata.quantity || 1,
			...metadata
		};

		// Track the revenue:purchased event
		sendCustomEvent('revenue:purchased', purchaseMetadata);

		log(`Registered purchase: ${priceInCents} cents (${currency})`);
	};

	const sendCustomEvent = (eventKey, data) => {
		if (!shouldTrack()) {
			return;
		}

		bufferEvent({
			url: window.location.href,
			timestamp: new Date().toISOString(),
			userId: window.Fusionaly.userId,
			eventType: window.Fusionaly.config.eventTypes.customEvent,
			eventMetadata: data,
			eventKey: eventKey,
		});
	};

	const setUser = (data) => {
		window.Fusionaly.userId = data.userId;
	};

	window.addEventListener("beforeunload", () => {
		if (!shouldTrack()) {
			return;
		}

		const eventsToSend = eventBuffer;
		eventBuffer = [];

		for (const eventData of eventsToSend) {
			eventData.userAgent = navigator.userAgent;

			const sent = navigator.sendBeacon(
				`${window.Fusionaly.config.host}/x/api/v1/events/beacon`,
				JSON.stringify(eventData),
			);
			if (!sent) {
				storeEventLocally(eventData);
			}
		}
	});

	const sanitizeForEventKey = (text) => {
		if (!text) return "unnamed_button";
		const sanitized = text
			.toLowerCase()
			.trim()
			.replace(/\s+/g, "_") // Replace whitespace with underscore
			.replace(/[^a-z0-9_:]/g, "") // Remove non-alphanumeric characters except underscore and colon
			.substring(0, 50); // Truncate to 50 chars
		return sanitized || "unnamed_button";
	};

	const formatEventSuffix = (value) => {
		if (value === undefined || value === null) {
			return "value";
		}
		let suffix = value;
		if (typeof suffix === "number") {
			const normalized = Number.isInteger(suffix)
				? suffix.toString()
				: suffix.toFixed(2);
			suffix = normalized;
		}
		suffix = suffix.toString().replace(/\./g, "_");
		const sanitized = sanitizeForEventKey(suffix);
		return sanitized || "value";
	};

	const buildEventKey = (base, suffix) => {
		const normalizedBase = (base || "event").trim().replace(/:+$/, "");
		const suffixSlug = formatEventSuffix(suffix);
		return `${normalizedBase}:${suffixSlug}`;
	};

	// Helper function to get data attribute
	const getDataAttribute = (element, attributeName) => {
		return element.getAttribute(`data-fusionaly-${attributeName}`);
	};

	const hasDataAttribute = (element, attributeName) => {
		return element.hasAttribute(`data-fusionaly-${attributeName}`);
	};

	const collectMetadataAttributes = (element, attributePrefix) => {
		const metadata = {};
		const fusionalyPrefix = `data-fusionaly-${attributePrefix}`;

		for (let i = 0; i < element.attributes.length; i++) {
			const attr = element.attributes[i];
			if (attr.name.startsWith(fusionalyPrefix)) {
				const key = attr.name.substring(fusionalyPrefix.length);
				if (key) {
					metadata[key] = attr.value;
				}
			}
		}

		return metadata;
	};

	// Helper function to process button events and extract event data
	const processButtonEvent = (button) => {
		let eventName = getDataAttribute(button, 'event-name');

		// If no custom event name, use the original auto-generated logic
		if (!eventName || eventName.trim() === "") {
			const buttonId = button.id || "noid";
			const buttonText = (
				button.textContent ||
				button.value ||
				button.title ||
				""
			).trim();

			const sanitizedName = sanitizeForEventKey(buttonText);
			eventName = `click:button:${sanitizedName}:${buttonId}`;
		}

		const metadata = {
			text: (button.textContent || button.value || button.title || "").trim(),
		};

		// Collect additional metadata from data attributes
		for (let i = 0; i < button.attributes.length; i++) {
			const attr = button.attributes[i];
			if (attr.name.startsWith('data-fusionaly-metadata-')) {
				const metadataKey = attr.name.substring('data-fusionaly-metadata-'.length);
				if (metadataKey) {
					metadata[metadataKey] = attr.value;
				}
			}
		}

		return { eventName, metadata };
	};

	const autoInstrumentButtons = () => {
		if (!shouldTrack()) {
			return;
		}

		document.addEventListener("click", (event) => {
			const button = event.target.closest(
				'button, [role="button"], input[type="button"], input[type="submit"]',
			);
			if (button) {
				// If this "button" is an <a> tag that will be handled by data-driven link tracking,
				// let that specific handler take precedence to avoid double tracking.
				if (button.tagName === 'A' && hasDataAttribute(button, 'event-name')) {
					return;
				}

				const eventData = processButtonEvent(button);
				sendCustomEvent(eventData.eventName, eventData.metadata);
			}
		});
	};

	const setupDataDrivenLinkTracking = () => {
		if (!shouldTrack()) {
			return;
		}

		document.addEventListener("click", (event) => {
			const link = event.target.closest('a[data-fusionaly-event-name]');

			if (link) {
				const href = link.getAttribute("href");
				// Skip processing if it's not a real navigation link
				if (!href || href === "#" || href.startsWith("javascript:")) {
					// Still track the event, but don't interfere with the link behavior
					processLinkEvent(link, href);
					return;
				}

				// For real navigation links, prevent the default behavior
				event.preventDefault();

				// Process the event and get the event data
				const eventData = processLinkEvent(link, href);

				// Use sendBeacon for the navigation case
				if (eventData) {
					// Get the original event name directly from the link attribute
					const originalEventName = getDataAttribute(link, 'event-name');

					// Prepare the event data with all required fields exactly matching the backend API structure
					const beaconData = {
						url: window.location.href,
						referrer: document.referrer || "",  // Ensure referrer is never undefined
						timestamp: new Date().toISOString(),
						userId: window.Fusionaly.userId || null,
						eventType: window.Fusionaly.config.eventTypes.customEvent,
						eventMetadata: eventData.metadata || {},  // Ensure metadata is never undefined
						eventKey: originalEventName,  // Use the original event name directly
						userAgent: navigator.userAgent
					};

					// For debugging - log the exact payload being sent
					if (window.Fusionaly.config.debug) {
						console.log('Fusionaly: Sending beacon payload:', beaconData);
					}

					// Send the beacon
					const sent = navigator.sendBeacon(
						`${window.Fusionaly.config.host}/x/api/v1/events/beacon`,
						JSON.stringify(beaconData)
					);

					if (!sent) {
						// If sendBeacon fails, fall back to storing the event locally
						log("sendBeacon failed, storing event locally", "warn");
						storeEventLocally(beaconData);
					}

					log(`Tracked link click with sendBeacon: ${originalEventName}`);
				}

				// Navigate to the link destination immediately after sending the beacon
				// Only open in new tab if target="_blank" is explicitly set
				if (link.target === "_blank") {
					window.open(href, "_blank");
				} else {
					// For all other cases, navigate in the same tab
					window.location.href = href;
				}
			}
		});
	};

	// Helper function to process link events and extract event data
	const processLinkEvent = (link, href) => {
		let eventName = getDataAttribute(link, 'event-name');
		if (!eventName || eventName.trim() === "") {
			log("Link click tracked with empty event name attribute, skipping.", "warn");
			return null;
		}

		const metadata = {
			href: href || "nohref",
			text: (link.textContent || link.innerText || "").trim(),
		};

		// Collect additional metadata from data attributes
		for (let i = 0; i < link.attributes.length; i++) {
			const attr = link.attributes[i];
			if (attr.name.startsWith('data-fusionaly-metadata-')) {
				const metadataKey = attr.name.substring('data-fusionaly-metadata-'.length);
				if (metadataKey) {
					metadata[metadataKey] = attr.value;
				}
			}
		}

		// For non-navigation links, send the event through the normal flow
		if (!href || href === "#" || href.startsWith("javascript:")) {
			// Use the original event name directly from the attribute
			const originalEventName = getDataAttribute(link, 'event-name');
			sendCustomEvent(originalEventName, metadata);
			log(`Tracked data-driven link click: ${originalEventName} with metadata: ${JSON.stringify(metadata)}`);
			return null;
		}

		// Return the event data for sendBeacon, using the original event name
		return { eventName: getDataAttribute(link, 'event-name'), metadata };
	};

	const normalizeIntersectionThreshold = (value, fallback) => {
		const clamp = (num) => Math.min(Math.max(num, 0.01), 1);

		if (typeof value === "number" && !Number.isNaN(value)) {
			if (value > 1) {
				return clamp(value / 100);
			}
			return clamp(value);
		}

		if (typeof value === "string" && value.trim() !== "") {
			const parsed = parseFloat(value);
			if (!Number.isNaN(parsed)) {
				if (parsed > 1) {
					return clamp(parsed / 100);
				}
				return clamp(parsed);
			}
		}

		return clamp(fallback);
	};

	const trackScrollSection = (target, options = {}) => {
		if (typeof IntersectionObserver === "undefined") {
			log("IntersectionObserver not supported; scroll section tracking disabled.", "warn");
			return () => {};
		}

		const element =
			typeof target === "string" ? document.querySelector(target) : target;

		if (!element) {
			log("trackScrollSection: target element not found.", "warn");
			return () => {};
		}

		if (options.__markElement && element.dataset.fusionalyScrollSectionTracked === "true") {
			return () => {};
		}

		const sectionName =
			typeof options.section === "string" && options.section.trim() !== ""
				? options.section
				: element.dataset.scrollSection ||
					element.getAttribute("id") ||
					getDataAttribute(element, "scroll-section") ||
					getDataAttribute(element, "track-scroll") ||
					"section";

		const baseEventName =
			options.eventName ||
			getDataAttribute(element, "scroll-event") ||
			window.Fusionaly.config.scrollSectionEventKey ||
			"scroll:section";

		const threshold = normalizeIntersectionThreshold(
			options.threshold || getDataAttribute(element, "scroll-threshold"),
			window.Fusionaly.config.scrollSectionThreshold || 0.5,
		);

		const once = options.once !== undefined ? options.once : true;
		const metadata = options.metadata || {};
		let hasFired = false;

		const observer = new IntersectionObserver(
			(entries, observerInstance) => {
				entries.forEach((entry) => {
					if (entry.isIntersecting && entry.intersectionRatio >= threshold) {
						if (once && hasFired) {
							return;
						}

						hasFired = true;
						const payload = {
							...metadata,
							section: sectionName,
						};
						const eventKey = buildEventKey(baseEventName, sectionName);
						sendCustomEvent(eventKey, payload);
						log(`Tracked scroll section: ${eventKey}`);

						if (once) {
							observerInstance.unobserve(entry.target);
						}
					}
				});
			},
			{
				threshold,
			},
		);

		observer.observe(element);

		if (options.__markElement) {
			element.dataset.fusionalyScrollSectionTracked = "true";
		}

		return () => {
			try {
				observer.unobserve(element);
			} catch (e) {
				/* no-op */
			}
		};
	};

	const parseScrollDepthThresholds = (input) => {
		if (input === undefined || input === null) {
			return [];
		}

		let values = [];

		if (Array.isArray(input)) {
			values = input;
		} else if (typeof input === "number") {
			values = [input];
		} else if (typeof input === "string") {
			values = input
				.split(/[, ]+/)
				.map((item) => item.trim())
				.filter((item) => item.length > 0);
		} else {
			return [];
		}

		const normalized = values
			.map((value) => {
				const numeric = typeof value === "number" ? value : parseFloat(value);
				if (Number.isNaN(numeric)) {
					return null;
				}

				if (numeric <= 0) {
					return null;
				}

				if (numeric <= 1) {
					return numeric * 100;
				}

				return numeric;
			})
			.filter((value) => value !== null)
			.map((value) => Math.min(Math.max(value, 1), 100));

		const unique = Array.from(new Set(normalized));
		return unique.sort((a, b) => a - b);
	};

	const trackScrollDepth = (thresholdInput, options = {}) => {
		if (typeof window === "undefined" || typeof document === "undefined") {
			return () => {};
		}

		let parsedThresholds = parseScrollDepthThresholds(thresholdInput);

		if (!parsedThresholds.length) {
			parsedThresholds = parseScrollDepthThresholds(
				window.Fusionaly.config.scrollDepthThresholds,
			);
		}

		if (!parsedThresholds.length) {
			parsedThresholds = [25, 50, 75, 100];
		}

		const thresholds = parsedThresholds;

		if (!thresholds.length) {
			log("trackScrollDepth: no valid thresholds provided.", "warn");
			return () => {};
		}

		const baseEventName =
			options.eventName ||
			window.Fusionaly.config.scrollDepthEventKey ||
			"scroll:depth";
		const metadata = options.metadata || {};
		const once = options.once !== undefined ? options.once : true;
		const reachedThresholds = new Set();
		let ticking = false;

		function cleanup() {
			window.removeEventListener("scroll", onScroll);
			window.removeEventListener("resize", onScroll);
		}

		function evaluateDepth() {
			const scrollTop = window.scrollY || window.pageYOffset || 0;
			const viewportHeight = window.innerHeight || 0;
			const doc = document.documentElement;
			const body = document.body;
			const docHeight = Math.max(
				doc.scrollHeight,
				body.scrollHeight,
				doc.offsetHeight,
				body.offsetHeight,
				doc.clientHeight,
			);

			const denominator = docHeight || viewportHeight || 1;
			const progress =
				denominator <= viewportHeight
					? 100
					: ((scrollTop + viewportHeight) / denominator) * 100;

			thresholds.forEach((threshold) => {
				if (!reachedThresholds.has(threshold) && progress >= threshold) {
					reachedThresholds.add(threshold);
					const payload = {
						...metadata,
						percentage: threshold,
					};
					const eventKey = buildEventKey(baseEventName, threshold);
					sendCustomEvent(eventKey, payload);
					log(`Tracked scroll depth: ${eventKey}`);
				}
			});

			if (once && reachedThresholds.size === thresholds.length) {
				cleanup();
			}

			ticking = false;
		}

		function onScroll() {
			if (ticking) {
				return;
			}
			ticking = true;
			requestAnimationFrame(evaluateDepth);
		}

		window.addEventListener("scroll", onScroll, { passive: true });
		window.addEventListener("resize", onScroll);
		evaluateDepth();

		return cleanup;
	};

	const setupScrollTrackingFromAttributes = () => {
		if (typeof document === "undefined") {
			return;
		}

		const sectionSelector = [
			"[data-fusionaly-scroll-section]",
			"[data-fusionaly-track-scroll]",
		].join(",");

		const sectionElements = document.querySelectorAll(sectionSelector);

		sectionElements.forEach((element) => {
			if (element.dataset.fusionalyScrollSectionTracked === "true") {
				return;
			}

			const sectionValue =
				getDataAttribute(element, "scroll-section") ||
				getDataAttribute(element, "track-scroll") ||
				element.dataset.scrollSection ||
				element.id ||
				"section";

			const eventName =
				getDataAttribute(element, "scroll-event") ||
				window.Fusionaly.config.scrollSectionEventKey ||
				"scroll:section";

			const threshold = getDataAttribute(element, "scroll-threshold");
			const metadata = collectMetadataAttributes(
				element,
				"scroll-metadata-",
			);

			trackScrollSection(element, {
				section: sectionValue,
				eventName,
				threshold,
				metadata,
				__markElement: true,
			});
		});

		const depthElements = document.querySelectorAll(
			"[data-fusionaly-track-scroll-depth]",
		);

		depthElements.forEach((depthElement) => {
			if (depthElement.dataset.fusionalyTrackScrollDepthTracked === "true") {
				return;
			}

			const thresholdsAttr = getDataAttribute(depthElement, "track-scroll-depth");
			const eventName =
				getDataAttribute(depthElement, "track-scroll-depth-event") ||
				window.Fusionaly.config.scrollDepthEventKey ||
				"scroll:depth";
			const metadata = collectMetadataAttributes(
				depthElement,
				"track-scroll-depth-metadata-",
			);

			trackScrollDepth(thresholdsAttr, {
				eventName,
				metadata,
			});

			depthElement.dataset.fusionalyTrackScrollDepthTracked = "true";
		});
	};

	// Initialize: set up SPA tracking and send initial page view
	setupSPATracking();
	if (window.Fusionaly.config.autoSendPageViews) {
		sendPageView();
	}
	if (window.Fusionaly.config.autoInstrumentButtons) {
		autoInstrumentButtons();
	}
	setupDataDrivenLinkTracking();
	setupScrollTrackingFromAttributes();

	if (document && typeof document.addEventListener === "function") {
		document.addEventListener("DOMContentLoaded", () => {
			// Re-run in case attributes were added late in the parse.
			setupScrollTrackingFromAttributes();
		});
	}

	// Initialize: check for stored events on load
	setTimeout(() => {
		if (isOnline) {
			sendStoredEvents();
		}
	}, 1000);

	window.Fusionaly.sendPageView =
		window.Fusionaly.sendPageView || sendPageView;
	window.Fusionaly.sendCustomEvent =
		window.Fusionaly.sendCustomEvent || sendCustomEvent;
	window.Fusionaly.setUser = window.Fusionaly.setUser || setUser;
	window.Fusionaly.registerPurchase = window.Fusionaly.registerPurchase || registerPurchase;
	window.Fusionaly.trackScrollDepth =
		window.Fusionaly.trackScrollDepth || trackScrollDepth;
	window.Fusionaly.trackScrollSection =
		window.Fusionaly.trackScrollSection || trackScrollSection;
	window.Fusionaly.setupScrollTrackingFromAttributes =
		window.Fusionaly.setupScrollTrackingFromAttributes ||
		setupScrollTrackingFromAttributes;

	setTimeout(sendBufferedEvents, window.Fusionaly.config.sendInterval);
})(window);
