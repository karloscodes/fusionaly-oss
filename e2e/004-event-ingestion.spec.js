import { test, expect } from "@playwright/test";

test.describe("Event Ingestion E2E", () => {
	let consoleErrors = [];
	let networkFailures = [];

	test.beforeEach(async ({ page }) => {
		// Reset error collectors
		consoleErrors = [];
		networkFailures = [];

		// Listen for console errors
		page.on("console", (msg) => {
			if (msg.type() === "error") {
				console.log(`Browser Console Error: ${msg.text()}`);
				consoleErrors.push(msg.text());
			}
		});

		// Listen for network request failures
		page.on("requestfailed", (request) => {
			console.log(
				`Network Request Failed: ${request.url()} ${request.failure()?.errorText}`,
			);
			networkFailures.push({
				url: request.url(),
				error: request.failure()?.errorText,
			});
		});
	});

	test("should handle beacon requests during page navigation", async ({ page }) => {
		// Navigate to the demo page
		await page.goto('/_demo');

		// Set up a promise for the beacon request before clicking
		const beaconResponsePromise = page.waitForResponse(
			async (response) => {
				if (!response.url().includes('/x/api/v1/events/beacon') || response.request().method() !== 'POST') {
					return false;
				}
				try {
					const data = await response.request().postDataJSON();
					return data.eventKey === 'click:link:google';
				} catch (e) {
					return false;
				}
			},
			{ timeout: 10000 }
		);

	// Click the link that should trigger the beacon
	await page.click('a[data-fusionaly-event-name="click:link:google"]');

		// Wait for and verify the beacon response
		const response = await beaconResponsePromise;
		expect(response.status(), "Beacon request should be Accepted (202)").toBe(202);

		// Get and verify the beacon request payload
		const postData = await response.request().postDataJSON();
		expect(postData).toHaveProperty('eventKey', 'click:link:google');
		expect(postData).toHaveProperty('eventType', 2); // CustomEvent type
		expect(postData).toHaveProperty('url');
		expect(postData).toHaveProperty('referrer');
		expect(postData).toHaveProperty('timestamp');
		expect(postData).toHaveProperty('userAgent');
		// Note: Origin validation replaced infinityCode - verification happens via Origin header
		expect(postData.eventMetadata).toHaveProperty('href');
		expect(postData.eventMetadata).toHaveProperty('text');

		// Verify no console errors
		expect(consoleErrors, "Expected no console errors").toEqual([]);

		// Verify no network failures for beacon requests
		const beaconFailures = networkFailures.filter(
			failure => failure.url.includes('/x/api/v1/events/beacon')
		);
		expect(beaconFailures, "Expected no beacon request failures").toEqual([]);
	});

	test("should support data-fusionaly attributes", async ({ page }) => {
		// Navigate to demo page first to have proper origin
		await page.goto('/_demo');

		// Wait for page to load
		await page.waitForLoadState('domcontentloaded');

		// Inject test elements with fusionaly attributes
		await page.evaluate(() => {
			document.body.innerHTML += `
				<a href="#" id="fusionaly-link" data-fusionaly-event-name="click:link:fusionaly" data-fusionaly-metadata-source="test">Fusionaly Link</a>
				<button id="fusionaly-button" data-fusionaly-event-name="click:button:fusionaly">Fusionaly Button</button>
			`;
		});

		// Wait for SDK to process new elements
		await page.waitForTimeout(200);

		// Test fusionaly-prefixed link
		const fusionalyLinkPromise = page.waitForRequest(
			request => {
				if (!request.url().includes('/x/api/v1/events') || request.method() !== 'POST') {
					return false;
				}
				try {
					const postData = request.postData();
					if (postData) {
						const data = JSON.parse(postData);
						return data.eventKey === 'click:link:fusionaly';
					}
				} catch (e) { }
				return false;
			},
			{ timeout: 5000 }
		);
		await page.click('#fusionaly-link');
		const fusionalyLinkRequest = await fusionalyLinkPromise;
		const fusionalyLinkData = JSON.parse(fusionalyLinkRequest.postData());
		expect(fusionalyLinkData.eventKey).toBe('click:link:fusionaly');
		expect(fusionalyLinkData.eventMetadata.source).toBe('test');

		// Test fusionaly-prefixed button
		const fusionalyButtonPromise = page.waitForRequest(
			request => {
				if (!request.url().includes('/x/api/v1/events') || request.method() !== 'POST') {
					return false;
				}
				try {
					const postData = request.postData();
					if (postData) {
						const data = JSON.parse(postData);
						return data.eventKey === 'click:button:fusionaly';
					}
				} catch (e) { }
				return false;
			},
			{ timeout: 5000 }
		);
		await page.click('#fusionaly-button');
		const fusionalyButtonRequest = await fusionalyButtonPromise;
		const fusionalyButtonData = JSON.parse(fusionalyButtonRequest.postData());
		expect(fusionalyButtonData.eventKey).toBe('click:button:fusionaly');

		// Verify no console errors
		expect(consoleErrors, "Expected no console errors").toEqual([]);
	});

	test("should emit scroll depth events via data attribute", async ({ page }) => {
		await page.goto('/_demo');
		await page.waitForLoadState('domcontentloaded');

	await page.evaluate(() => {
		document.body.setAttribute('data-fusionaly-track-scroll-depth', '25,50');
		document.body.setAttribute('data-fusionaly-track-scroll-depth-event', 'scroll:depth');
		document.body.setAttribute('data-fusionaly-track-scroll-depth-metadata-page', 'demo');
		const filler = document.createElement('div');
		filler.style.height = '5000px';
		document.body.appendChild(filler);
		if (window.Fusionaly?.setupScrollTrackingFromAttributes) {
			window.Fusionaly.setupScrollTrackingFromAttributes();
		}
	});

	const depthRequestPromise = page.waitForRequest((request) => {
		if (!request.url().includes('/x/api/v1/events') || request.method() !== 'POST') {
			return false;
		}
		try {
			const data = JSON.parse(request.postData() || '{}');
			return data.eventKey === 'scroll:depth:25' || data.eventKey === 'scroll:depth:50';
		} catch (error) {
			return false;
		}
	}, { timeout: 10_000 });

		await page.evaluate(() => {
			window.scrollTo({ top: document.body.scrollHeight, behavior: 'instant' });
		});

	const depthRequest = await depthRequestPromise;
	const depthData = JSON.parse(depthRequest.postData());
	expect(['scroll:depth:25', 'scroll:depth:50']).toContain(depthData.eventKey);
	expect(depthData.eventMetadata.page).toBe('demo');
	expect(depthData.eventMetadata.percentage).toBeDefined();
	});

	test("should emit scroll section events via data attribute", async ({ page }) => {
		await page.goto('/_demo');
		await page.waitForLoadState('domcontentloaded');

	await page.evaluate(() => {
		const spacer = document.createElement('div');
		spacer.style.height = '2000px';
		document.body.appendChild(spacer);

		const section = document.createElement('section');
		section.id = 'cta-section';
		section.style.height = '600px';
		section.setAttribute('data-fusionaly-scroll-section', 'cta');
		section.setAttribute('data-fusionaly-scroll-event', 'scroll:section');
		section.setAttribute('data-fusionaly-scroll-threshold', '0.4');
		section.setAttribute('data-fusionaly-scroll-metadata-source', 'demo');
		document.body.appendChild(section);

			const tail = document.createElement('div');
			tail.style.height = '2000px';
			document.body.appendChild(tail);

			if (window.Fusionaly?.setupScrollTrackingFromAttributes) {
				window.Fusionaly.setupScrollTrackingFromAttributes();
			}
		});

	const sectionRequestPromise = page.waitForRequest((request) => {
		if (!request.url().includes('/x/api/v1/events') || request.method() !== 'POST') {
			return false;
		}
		try {
			const data = JSON.parse(request.postData() || '{}');
			return data.eventKey === 'scroll:section:cta';
		} catch (error) {
			return false;
		}
	}, { timeout: 10_000 });

		await page.evaluate(() => {
			document.getElementById('cta-section')?.scrollIntoView({ behavior: 'instant' });
		});

	const sectionRequest = await sectionRequestPromise;
	const sectionData = JSON.parse(sectionRequest.postData());
	expect(sectionData.eventKey).toBe('scroll:section:cta');
	expect(sectionData.eventMetadata.section).toBe('cta');
	expect(sectionData.eventMetadata.source).toBe('demo');
	});

	test("should ingest page view, custom event, and purchase registration without errors", async ({
		page,
	}) => {
		// --- Promises for API Requests ---

		// Promise waits for the Page View event POST request
		const pageViewRequestPromise = page.waitForResponse(
			async (response) => {
				if (
					!response.url().includes("/x/api/v1/events") ||
					response.request().method() !== "POST"
				) {
					return false;
				}
				try {
					const body = await response.request().postDataJSON();
					// Identify based on eventType 1 (PageView)
					return body.eventType === 1;
				} catch (e) {
					console.error("Error parsing request body for PageView:", e);
					return false;
				}
			},
			{ timeout: 15000 }, // Increased timeout
		);

		// Promise waits for the Custom Event POST request
		const customEventRequestPromise = page.waitForResponse(
			async (response) => {
				if (
					!response.url().includes("/x/api/v1/events") ||
					response.request().method() !== "POST"
				) {
					return false;
				}
				try {
					const body = await response.request().postDataJSON();
					// Identify based on eventType 2 (CustomEvent) and the specific key
					return body.eventType === 2 && body.eventKey === "user:subscribed";
				} catch (e) {
					console.error("Error parsing request body for CustomEvent:", e);
					return false;
				}
			},
			{ timeout: 15000 }, // Increased timeout
		);

		// Promise waits for the Purchase Registration event POST request
		const purchaseRequestPromise = page.waitForResponse(
			async (response) => {
				if (
					!response.url().includes("/x/api/v1/events") ||
					response.request().method() !== "POST"
				) {
					return false;
				}
				try {
					const body = await response.request().postDataJSON();
					// Identify based on eventType 2 (CustomEvent) and the specific key "revenue:purchased"
					return body.eventType === 2 && body.eventKey === "revenue:purchased";
				} catch (e) {
					console.error("Error parsing request body for Purchase:", e);
					return false;
				}
			},
			{ timeout: 15000 }, // Increased timeout
		);

		// --- Navigate and Wait ---

		// Navigate to the demo page which includes the SDK and triggers all events
		await page.goto("/_demo");
		await page.waitForLoadState("networkidle");

		// --- Verify Responses ---

		let pageViewResponse;
		let customEventResponse;
		let purchaseResponse;

		try {
			// Wait for all three requests to complete
			[pageViewResponse, customEventResponse, purchaseResponse] = await Promise.all([
				pageViewRequestPromise,
				customEventRequestPromise,
				purchaseRequestPromise,
			]);
		} catch (error) {
			// If Promise.all fails (timeout), log all network requests for debugging
			console.log("Timeout waiting for event requests. Logging all network requests made:");
			const allRequests = [];
			page.on('request', request => {
				allRequests.push({
					url: request.url(),
					method: request.method(),
					postData: request.postData()
				});
			});
			console.log(JSON.stringify(allRequests, null, 2));

			// Throw the error to fail the test directly
			throw new Error(
				`Timeout waiting for one or more event requests: ${error}`,
			);
		}

		// --- Assert Status and Body AFTER requests completed ---
		console.log(
			`PageView Event request completed with status ${pageViewResponse.status()}`,
		);
		expect(
			pageViewResponse.status(),
			"PageView request should be Accepted (202)",
		).toBe(202);
		const pvResponseBody = await pageViewResponse.json();
		expect(
			pvResponseBody.message,
			"PageView response message should be correct",
		).toBe("Event added successfully");
		expect(
			pvResponseBody.status,
			"PageView response status in body should be correct",
		).toBe(202);

		console.log(
			`Custom Event request completed with status ${customEventResponse.status()}`,
		);
		expect(
			customEventResponse.status(),
			"Custom Event request should be Accepted (202)",
		).toBe(202);
		const ceResponseBody = await customEventResponse.json();
		expect(
			ceResponseBody.message,
			"Custom Event response message should be correct",
		).toBe("Event added successfully");
		expect(
			ceResponseBody.status,
			"Custom Event response status in body should be correct",
		).toBe(202);

		console.log(
			`Purchase Event request completed with status ${purchaseResponse.status()}`,
		);
		expect(
			purchaseResponse.status(),
			"Purchase request should be Accepted (202)",
		).toBe(202);
		const purchaseResponseBody = await purchaseResponse.json();
		expect(
			purchaseResponseBody.message,
			"Purchase response message should be correct",
		).toBe("Event added successfully");
		expect(
			purchaseResponseBody.status,
			"Purchase response status in body should be correct",
		).toBe(202);

		// Wait a little longer just in case of delayed errors
		await page.waitForTimeout(500);

		// --- Final Assertions ---
		expect(consoleErrors, "Expected no console errors").toEqual([]);

		// Filter network failures to relevant ones (SDK or API)
		const relevantNetworkFailures = networkFailures.filter(
			(failure) =>
				failure.url.includes("sdk.js") ||
				failure.url.includes("/x/api/v1/events"),
		);
		expect(
			relevantNetworkFailures,
			"Expected no relevant network failures",
		).toEqual([]);
	});
});
