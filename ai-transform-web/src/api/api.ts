import request from '../request/axios';

export interface record {
    id:number,
    project_name:string,
    original_language:string,
    translated_language:string,
    original_video_url:string,
    translated_video_url:string,
    expiration_at:number,
    create_at:number,
}

export function getTranslateRecords() {
    let path = '/v1/records'
    return request.get(path)
}


export interface transInfo {
    project_name:string,
    original_language:string,
    translate_language:string,    
    file_url:string,
}

export function translate(params:transInfo) {
    let path = '/v1/translate'
    let formData = new FormData()
    formData.append("project_name",params.project_name)
    formData.append("original_language",params.original_language)
    formData.append("translate_language",params.translate_language)
    formData.append("file_url",params.file_url)
    return request.post(path,formData)
}

export function cosPresignedUrl(filename:string){
    let path = '/v1/cos/presigned/url'
    return request.get(path + "?filename=" + filename)
}
export function uploadCos(presignedUrl:string,fileContent: ArrayBuffer) {
    return request.put(presignedUrl, fileContent)
}