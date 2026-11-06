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
	        this.preview_data_url = source["preview_data_url"];
	        this.capture_previews = source["capture_previews"];
	        this.demo_mode = source["demo_mode"];
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

}
