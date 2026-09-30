import { PublicDegree } from '@/components/common/degree-portal'

export default async function Page({params}:{params:Promise<{id:string}>}) {
 const {id}=await params
 return <PublicDegree id={id}/>
}
