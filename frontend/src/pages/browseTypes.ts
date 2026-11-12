export interface ComponentAsset {
  id: string;
  path: string;
  fileSize: number;
  contentType: string;
  repository?: string;
  createdAt?: string;
  lastModified?: string;
  lastDownloaded?: string | null;
  sha256?: string;
  sha1?: string;
  md5?: string;
}
export interface Component {
  id: string;
  name: string;
  group: string;
  version: string;
  format: string;
  repository?: string;
  createdAt?: string;
  lastDownloaded?: string | null;
  assets?: ComponentAsset[];
}
