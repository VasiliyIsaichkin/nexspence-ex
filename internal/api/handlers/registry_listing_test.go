package handlers_test

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http/httptest"
 "testing"

 "github.com/gin-gonic/gin"
 "github.com/nexspence-oss/nexspence/internal/api/handlers"
 "github.com/nexspence-oss/nexspence/internal/domain"
 "github.com/nexspence-oss/nexspence/internal/formats"
 "github.com/nexspence-oss/nexspence/internal/formats/oci"
 "github.com/nexspence-oss/nexspence/internal/repository"
 "github.com/nexspence-oss/nexspence/internal/service"
 "github.com/nexspence-oss/nexspence/internal/testutil"
 "github.com/stretchr/testify/require"
 "go.uber.org/zap"
)

func TestRegistryScopedListings(t *testing.T){
 repo:=&domain.Repository{ID:"r",Name:"docker-hosted",Format:domain.FormatDocker,Type:domain.TypeHosted,Online:true}
 repos:=testutil.NewRepoRepo(repo);assets:=testutil.NewAssetRepo();comps:=testutil.NewComponentRepo()
 grants:=&privsRBACRepo{privs:[]repository.PrivilegeWithSelector{{Actions:[]string{"read","browse","write"},Expression:`repository == "docker-hosted" && path.startsWith("/team-b/")`}}}
 rbac:=service.NewRBACService(grants,repos,zap.NewNop().Sugar(),true)
 rows:=[]domain.DockerBrowseRow{}
 for i:=0;i<450;i++{name:=fmt.Sprintf("team-a/image-%03d",i);rows=append(rows,domain.DockerBrowseRow{ComponentID:fmt.Sprint(i),ImageName:name,Version:"1.0.0"});require.NoError(t,assets.Create(context.Background(),&domain.Asset{Repository:repo.Name,Path:"/manifests/"+name+"/1.0.0"}))}
 for _,name:=range []string{"team-b/a","team-b/b","team-b/c"}{rows=append(rows,domain.DockerBrowseRow{ComponentID:name,ImageName:name,Version:"1.0.0"});require.NoError(t,assets.Create(context.Background(),&domain.Asset{Repository:repo.Name,Path:"/manifests/"+name+"/1.0.0"}))}
 comps.DockerRowsByRepo=map[string][]domain.DockerBrowseRow{repo.Name:rows}
 deps:=formats.Deps{Repos:repos,Assets:assets,Components:comps,RBAC:rbac}
 r:=gin.New();r.Use(func(c *gin.Context){c.Set("userID","team-b");c.Set("roles",[]string{});c.Next()})
 browse:=handlers.NewBrowseHandler(deps,rbac)
 r.GET("/browse/:name",browse.DockerTree)
 r.Any("/repository/:repoName/*path",handlers.RBACMiddleware(rbac,repos),oci.New(deps).ServeHTTP)
 for path,want:=range map[string]int{"/browse/docker-hosted?limit=1":200,"/repository/docker-hosted/v2/_catalog?n=2":200,"/repository/docker-hosted/v2/team-a/image-000/manifests/1.0.0":403}{
  w:=httptest.NewRecorder();r.ServeHTTP(w,httptest.NewRequest("GET",path,nil));require.Equal(t,want,w.Code,w.Body.String())
  if want==200 {require.NotContains(t,w.Body.String(),"team-a")}
  if path=="/repository/docker-hosted/v2/_catalog?n=2"{var doc struct{Repositories []string};require.NoError(t,json.Unmarshal(w.Body.Bytes(),&doc));require.Equal(t,[]string{"team-b/a","team-b/b"},doc.Repositories);require.Contains(t,w.Header().Get("Link"),"last=team-b%2Fb")}
 }
}
