export namespace main {
	
	export class CameraPreviewStatus {
	    state: string;
	    message: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CameraPreviewStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.message = source["message"];
	        this.available = source["available"];
	    }
	}
	export class DemoSession {
	    id: string;
	    state: string;
	    captured: number;
	    total: number;
	    failure_reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new DemoSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.captured = source["captured"];
	        this.total = source["total"];
	        this.failure_reason = source["failure_reason"];
	    }
	}
	export class PlayerFramePackage {
	    package_id: string;
	    package_version: string;
	    name: string;
	    description?: string;
	    author: string;
	    license: string;
	    frame_path?: string;
	    preview_path?: string;
	    slot_count: number;
	    has_preview: boolean;
	    already_installed?: boolean;
	    is_active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerFramePackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.package_id = source["package_id"];
	        this.package_version = source["package_version"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.license = source["license"];
	        this.frame_path = source["frame_path"];
	        this.preview_path = source["preview_path"];
	        this.slot_count = source["slot_count"];
	        this.has_preview = source["has_preview"];
	        this.already_installed = source["already_installed"];
	        this.is_active = source["is_active"];
	    }
	}
	export class PlayerFramePackageState {
	    packages: PlayerFramePackage[];
	    active_frame: string;
	    can_rollback: boolean;
	    load_warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayerFramePackageState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packages = this.convertValues(source["packages"], PlayerFramePackage);
	        this.active_frame = source["active_frame"];
	        this.can_rollback = source["can_rollback"];
	        this.load_warning = source["load_warning"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlayerOutputTargetView {
	    name: string;
	    status: string;
	    job_id?: string;
	    error?: string;
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new PlayerOutputTargetView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.job_id = source["job_id"];
	        this.error = source["error"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class PlayerSession {
	    id: string;
	    output_retrying?: boolean;
	    state: string;
	    captured: number;
	    total: number;
	    current_shot: number;
	    countdown_seconds: number;
	    message: string;
	    error_message?: string;
	    camera_mode: string;
	    output_path?: string;
	    rendered_path?: string;
	    output_targets?: PlayerOutputTargetView[];
	    preview_data_url?: string;
	    capture_previews?: string[];
	    demo_mode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.output_retrying = source["output_retrying"];
	        this.state = source["state"];
	        this.captured = source["captured"];
	        this.total = source["total"];
	        this.current_shot = source["current_shot"];
	        this.countdown_seconds = source["countdown_seconds"];
	        this.message = source["message"];
	        this.error_message = source["error_message"];
	        this.camera_mode = source["camera_mode"];
	        this.output_path = source["output_path"];
	        this.rendered_path = source["rendered_path"];
	        this.output_targets = this.convertValues(source["output_targets"], PlayerOutputTargetView);
	        this.preview_data_url = source["preview_data_url"];
	        this.capture_previews = source["capture_previews"];
	        this.demo_mode = source["demo_mode"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlayerSettings {
	    shot_count: number;
	    countdown: string;
	    interval: string;
	    session_retention_days: number;
	    output_retention_days: number;
	    frame_path: string;
	    config_path: string;
	    load_warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayerSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shot_count = source["shot_count"];
	        this.countdown = source["countdown"];
	        this.interval = source["interval"];
	        this.session_retention_days = source["session_retention_days"];
	        this.output_retention_days = source["output_retention_days"];
	        this.frame_path = source["frame_path"];
	        this.config_path = source["config_path"];
	        this.load_warning = source["load_warning"];
	    }
	}
	export class PrintAttemptView {
	    attempt_id: string;
	    queue_name: string;
	    document_name: string;
	    source_sha256: string;
	    status: string;
	    job_id?: string;
	    error?: string;
	    requested_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintAttemptView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attempt_id = source["attempt_id"];
	        this.queue_name = source["queue_name"];
	        this.document_name = source["document_name"];
	        this.source_sha256 = source["source_sha256"];
	        this.status = source["status"];
	        this.job_id = source["job_id"];
	        this.error = source["error"];
	        this.requested_at = source["requested_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class PrintLockReconciliationView {
	    attempt_id: string;
	    status: string;
	    created: boolean;
	    removed: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintLockReconciliationView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attempt_id = source["attempt_id"];
	        this.status = source["status"];
	        this.created = source["created"];
	        this.removed = source["removed"];
	        this.message = source["message"];
	    }
	}
	export class PrintLockView {
	    lease_version: number;
	    owner_pid?: number;
	    attempt_id: string;
	    queue_name: string;
	    document_name: string;
	    source_sha256?: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintLockView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lease_version = source["lease_version"];
	        this.owner_pid = source["owner_pid"];
	        this.attempt_id = source["attempt_id"];
	        this.queue_name = source["queue_name"];
	        this.document_name = source["document_name"];
	        this.source_sha256 = source["source_sha256"];
	        this.created_at = source["created_at"];
	    }
	}
	export class PrintResolutionView {
	    attempt_id: string;
	    decision: string;
	    queue_found: boolean;
	    queue_checked_at: string;
	    recorded_at: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintResolutionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attempt_id = source["attempt_id"];
	        this.decision = source["decision"];
	        this.queue_found = source["queue_found"];
	        this.queue_checked_at = source["queue_checked_at"];
	        this.recorded_at = source["recorded_at"];
	        this.note = source["note"];
	    }
	}
	export class PrinterQueueJobView {
	    job_id: string;
	    document_name: string;
	    driver_status?: string;
	    status_flags: number;
	    status?: string[];
	    position: number;
	    total_pages: number;
	    pages_printed: number;
	
	    static createFrom(source: any = {}) {
	        return new PrinterQueueJobView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.document_name = source["document_name"];
	        this.driver_status = source["driver_status"];
	        this.status_flags = source["status_flags"];
	        this.status = source["status"];
	        this.position = source["position"];
	        this.total_pages = source["total_pages"];
	        this.pages_printed = source["pages_printed"];
	    }
	}
	export class PrinterInspectionView {
	    queue_name: string;
	    found: boolean;
	    inspected_at: string;
	    jobs?: PrinterQueueJobView[];
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new PrinterInspectionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.queue_name = source["queue_name"];
	        this.found = source["found"];
	        this.inspected_at = source["inspected_at"];
	        this.jobs = this.convertValues(source["jobs"], PrinterQueueJobView);
	        this.note = source["note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RecoveryManifestView {
	    expected_shots: number;
	    demo_mode: boolean;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new RecoveryManifestView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.expected_shots = source["expected_shots"];
	        this.demo_mode = source["demo_mode"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class RecoveryItemView {
	    session_id: string;
	    directory_path: string;
	    status: string;
	    reason?: string;
	    completed_shots: number;
	    expected_shots: number;
	    phase?: string;
	    manifest?: RecoveryManifestView;
	    capture_paths?: string[];
	    rendered_path?: string;
	    print_attempts?: PrintAttemptView[];
	    print_resolutions?: PrintResolutionView[];
	    print_lock_present?: boolean;
	    print_lock_owner?: string;
	    print_lock?: PrintLockView;
	
	    static createFrom(source: any = {}) {
	        return new RecoveryItemView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.directory_path = source["directory_path"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.completed_shots = source["completed_shots"];
	        this.expected_shots = source["expected_shots"];
	        this.phase = source["phase"];
	        this.manifest = this.convertValues(source["manifest"], RecoveryManifestView);
	        this.capture_paths = source["capture_paths"];
	        this.rendered_path = source["rendered_path"];
	        this.print_attempts = this.convertValues(source["print_attempts"], PrintAttemptView);
	        this.print_resolutions = this.convertValues(source["print_resolutions"], PrintResolutionView);
	        this.print_lock_present = source["print_lock_present"];
	        this.print_lock_owner = source["print_lock_owner"];
	        this.print_lock = this.convertValues(source["print_lock"], PrintLockView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RetentionApplyResult {
	    removed_candidates: number;
	    removed_files: number;
	    removed_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new RetentionApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removed_candidates = source["removed_candidates"];
	        this.removed_files = source["removed_files"];
	        this.removed_bytes = source["removed_bytes"];
	    }
	}
	export class RetentionCandidateView {
	    kind: string;
	    session_id: string;
	    path: string;
	    last_modified: string;
	    file_count: number;
	    size_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new RetentionCandidateView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.session_id = source["session_id"];
	        this.path = source["path"];
	        this.last_modified = source["last_modified"];
	        this.file_count = source["file_count"];
	        this.size_bytes = source["size_bytes"];
	    }
	}
	export class RetentionPreview {
	    generated_at: string;
	    disabled: boolean;
	    session_retention_days: number;
	    output_retention_days: number;
	    scanned_count: number;
	    skipped_count: number;
	    candidate_count: number;
	    total_files: number;
	    total_bytes: number;
	    plan_digest: string;
	    truncated: boolean;
	    candidates: RetentionCandidateView[];
	
	    static createFrom(source: any = {}) {
	        return new RetentionPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generated_at = source["generated_at"];
	        this.disabled = source["disabled"];
	        this.session_retention_days = source["session_retention_days"];
	        this.output_retention_days = source["output_retention_days"];
	        this.scanned_count = source["scanned_count"];
	        this.skipped_count = source["skipped_count"];
	        this.candidate_count = source["candidate_count"];
	        this.total_files = source["total_files"];
	        this.total_bytes = source["total_bytes"];
	        this.plan_digest = source["plan_digest"];
	        this.truncated = source["truncated"];
	        this.candidates = this.convertValues(source["candidates"], RetentionCandidateView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SMBAdminInterface {
	    interface_alias: string;
	    description: string;
	    network_profile: string;
	    local_addresses: string[];
	
	    static createFrom(source: any = {}) {
	        return new SMBAdminInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interface_alias = source["interface_alias"];
	        this.description = source["description"];
	        this.network_profile = source["network_profile"];
	        this.local_addresses = source["local_addresses"];
	    }
	}
	export class SMBAdminStatus {
	    service_state: string;
	    service_start_mode: string;
	    auto_share_wks_configured: boolean;
	    auto_share_wks: number;
	    allow_rule_present: boolean;
	    allow_local_addresses: string[];
	    allowed_remote_addresses: string[];
	    smb_block_rule_active: boolean;
	    netbios_tcp_block_rule_active: boolean;
	    netbios_udp_block_rule_active: boolean;
	    listener_count: number;
	
	    static createFrom(source: any = {}) {
	        return new SMBAdminStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.service_state = source["service_state"];
	        this.service_start_mode = source["service_start_mode"];
	        this.auto_share_wks_configured = source["auto_share_wks_configured"];
	        this.auto_share_wks = source["auto_share_wks"];
	        this.allow_rule_present = source["allow_rule_present"];
	        this.allow_local_addresses = source["allow_local_addresses"];
	        this.allowed_remote_addresses = source["allowed_remote_addresses"];
	        this.smb_block_rule_active = source["smb_block_rule_active"];
	        this.netbios_tcp_block_rule_active = source["netbios_tcp_block_rule_active"];
	        this.netbios_udp_block_rule_active = source["netbios_udp_block_rule_active"];
	        this.listener_count = source["listener_count"];
	    }
	}

}

